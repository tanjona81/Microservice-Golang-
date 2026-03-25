package main

import (
	"context"
	"errors"
	"example/hello/internal/config"
	"example/hello/internal/dbconfig"
	"example/hello/internal/domain"
	"example/hello/internal/handlers"
	"example/hello/internal/middleware"
	"example/hello/internal/repositories"
	"example/hello/internal/services"
	"example/hello/internal/utils"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
)

func main() {
	// err := godotenv.Load()
	// if err != nil {
	// 	log.Println("No .env file found, reading from system environment")
	// }

	// Config
	appConfg := config.LoadConfig()

	if err := appConfg.Validate(); err != nil {
		slog.Error("Invalid Environment Variable Configuration", "details", err)
		os.Exit(1)
	}

	slog.Debug("Logger initialized", "env_value", appConfg.AppEnv)
	slog.Debug("Database name check", "env_value DB_NAME", appConfg.Database.Name)
	slog.Debug("JUST HERE")

	// Seting up utilities
	utils.SetSecretKey(appConfg.JwtSecretKey)
	utils.SetupErrors(appConfg.CircuitBreak.Timeout)

	// Setup Database
	// appDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
	// 	os.Getenv("DB_USER"),
	// 	os.Getenv("DB_PASSWORD"),
	// 	os.Getenv("DB_HOST"),
	// 	os.Getenv("DB_PORT"),
	// 	os.Getenv("DB_NAME"),
	// 	os.Getenv("DB_MIGRATE_PARAMS"), // Use the safe version
	// )
	dbConn, err := dbconfig.ConnectDB(appConfg.Database.DSN)
	if err != nil {
		log.Fatal("[CRITICAL] Could not connect to DB:", err)
	}

	defer func() {
		log.Println("Closing database connection...")
		dbConn.Close()
	}()

	// Run migrations before anything else starts
	dbconfig.RunMigrations(dbConn)

	// Setup redis
	redisdb := redis.NewClient(&redis.Options{
		Addr:     appConfg.Redis.RedisAddr,
		Password: appConfg.Redis.RedisPass,
	})

	// Check if Redis Shield is actually active
	// We use a context with timeout so the app doesn't hang forever if Docker is off
	redisCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := redisdb.Ping(redisCtx).Err(); err != nil {
		log.Fatalf("[CRITICAL] Redis shield not found. Error: %v", err)
	}
	log.Println("Redis shield active and connected.")

	// If the DB fails 5 times in a row, "Trip" the circuit. Stop sending requests for 30 seconds to let the DB recover.
	settings := gobreaker.Settings{
		Name:        "Database",
		MaxRequests: uint32(appConfg.CircuitBreak.MaxRequests), // Max requests allowed when Half-Open
		Interval:    appConfg.CircuitBreak.Interval,            // Period to clear counts
		Timeout:     appConfg.CircuitBreak.TimeoutSeconds,      // How long to stay Open (Red)
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Don't trip on a single failure; wait for a sample size of at least 5
			if counts.Requests < 5 {
				return false
			}

			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)

			// Trip if failure rate > 60% OR if we hit 5 consecutive failures
			return failureRatio > appConfg.CircuitBreak.FailureRatio || counts.ConsecutiveFailures > uint32(appConfg.CircuitBreak.ConsecutiveFailure)
		},
		// Define what actually counts as a "failure"
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}

			// Unwrap the error if it's wrapped
			unwrapped := errors.Unwrap(err)
			if unwrapped == nil {
				unwrapped = err
			}

			// LOGIC ERRORS: Do not count these as system failures.
			switch unwrapped {
			case domain.ErrNotFound,
				domain.ErrInvalidCredentials,
				domain.ErrInvalidInput,
				domain.ErrTokenExpired,
				domain.ErrUnauthorized,
				domain.ErrConflict,
				domain.ErrCompromisedSession,
				domain.ErrDuplicateEmail:
				return true
			}

			// Only return false for "System" errors (Timeout, Connection Refused, etc.)
			return false
		},
	}
	dbBreaker := gobreaker.NewCircuitBreaker(settings)

	// Dependency Injection Chain
	// Repositories
	userRepo := repositories.NewUserRepository(dbConn)
	authRepo := repositories.NewAuthRepository(dbConn)
	tokenRepo := repositories.NewTokenRepository(dbConn)

	// Services
	userService := services.NewUserService(dbConn, dbBreaker, userRepo, redisdb)
	tokenService := services.NewTokenService(dbConn, dbBreaker, tokenRepo, userRepo, authRepo, appConfg)
	authService := services.NewAuthService(dbConn, dbBreaker, authRepo, userRepo, tokenRepo, tokenService, appConfg, redisdb)

	// Handlers
	userHdl := handlers.NewUserHandler(appConfg, userService)
	authHdl := handlers.NewAuthHandler(appConfg, authService, tokenService)

	// Create the router
	router := chi.NewRouter()

	// Prevent crashes
	router.Use(middleware.Recovery)

	// CORS
	router.Use(middleware.CORS)

	// RequestID
	router.Use(middleware.RequestID)

	// Global logger Middleware
	router.Use(middleware.Logger)

	router.Route("/metrics", func(r chi.Router) {
		// Note: Use .Handle here because promhttp.Handler() returns an http.Handler
		r.Handle("/", promhttp.Handler())
	})

	router.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(appConfg.Timeout.Auth))

		// LOGIN: Protected against password guessing
		r.Group(func(r chi.Router) {
			r.Use(middleware.BruteForceMiddleware(redisdb, appConfg))
			r.Post("/api/v1/auth/login", authHdl.Login)
		})

		// REGISTER: Protected against spamming (Standard Rate Limit)
		r.Group(func(r chi.Router) {
			// Use a lighter weight limiter here if you have one,
			// or just the standard timeout for now.
			r.Use(middleware.ShadowLimiter(redisdb, appConfg))
			r.Post("/api/v1/auth/register", authHdl.Register)
		})

		// LOGOUT & REFRESH
		r.Post("/api/v1/auth/logout", authHdl.Logout)
		r.Post("/api/v1/auth/refresh", authHdl.Refresh)
	})

	router.Route("/api/v1/users", func(r chi.Router) {
		// Apply timeout for persistance
		r.Use(middleware.Timeout(appConfg.Timeout.Standard))

		// No ID needed
		r.Get("/", userHdl.GetUsersCursorHandler)
		r.Get("/offset", userHdl.GetUsersOffsetHandler)
		r.Post("/", userHdl.CreateUserHandler)

		r.Route("/grpc/{id}", func(r chi.Router) {
			r.Use(middleware.UserIDCtx)
			r.Get("/", userHdl.GetUserFromGRPC) // A gRPC call
		})

		// ID REQUIRED
		r.Route("/{id}", func(r chi.Router) {
			// Runs for everything in this block
			r.Use(middleware.Auth)
			r.Use(middleware.UserIDCtx)

			r.Group(func(r chi.Router) {
				r.Use(middleware.EnsureRole("user"))         // Users see themselves
				r.Get("/", userHdl.GetUsersByIDHandler)      // GET /users/10
				r.Put("/", userHdl.PutUpdateUserHandler)     // PUT /users/10
				r.Patch("/", userHdl.PatchUpdateUserHandler) // PATCH /users/10
				r.Delete("/", userHdl.DeleteUserHandler)     // DELETE /users/10
			})

		})
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Run server in a goroutine so it doesn't block
	go func() {
		log.Println("Server starting on :8080...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[CRITICAL] listen: %s\n", err)
		}
	}()

	// Create a channel to listen for OS signals (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Wait here until a signal is received
	<-quit
	log.Println("Shutdown signal received. Shutting down gracefully...")

	// Create a timeout context for the shutdown
	// This gives the server 5 seconds to finish active requests
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Trigger the shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting. Goodbye")

}
