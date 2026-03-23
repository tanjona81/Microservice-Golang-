package services

import (
	"context"
	"database/sql"
	"errors"
	"example/hello/internal/config"
	"example/hello/internal/dbconfig"
	"example/hello/internal/domain"
	"example/hello/internal/dto"
	"example/hello/internal/models"
	"example/hello/internal/repositories"
	"example/hello/internal/utils"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
)

// Define the Interface (The Contract)
type AuthService interface {
	Login(ctx context.Context, email string, password string, ip string) (*dto.LoginResult, error)
	Register(ctx context.Context, req dto.CreateUserRequest) (*dto.RegisterResult, error)
}

// Define the Struct (The Implementation)
type authService struct {
	db              *sql.DB
	breaker         *gobreaker.CircuitBreaker
	repository      repositories.AuthRepository
	userRepository  repositories.UserRepository
	tokenRepository repositories.TokenRepository
	tokenService    TokenService
	config          *config.Config
	redisClient     *redis.Client
}

// The Constructor (How to create the service)
func NewAuthService(
	db *sql.DB,
	b *gobreaker.CircuitBreaker,
	repo repositories.AuthRepository,
	userRepo repositories.UserRepository,
	tokenRepo repositories.TokenRepository,
	tokenSvc TokenService,
	c *config.Config,
	redis *redis.Client,
) AuthService {
	if b == nil {
		log.Fatal("AuthService requires a non-nil CircuitBreaker")
	}
	return &authService{
		db:              db,
		breaker:         b,
		repository:      repo,
		userRepository:  userRepo,
		tokenRepository: tokenRepo,
		tokenService:    tokenSvc,
		config:          c,
		redisClient:     redis,
	}
}

// Internal helper to keep Login() readable
func (service *authService) recordStrike(ctx context.Context, key string, duration time.Duration) {
	_, err := utils.CreatePipeline(service.redisClient, ctx, key, duration)
	if err != nil {
		slog.Error("RateLimiter: Redis Limiter Error", "err", err)
	}
}

func (service *authService) Login(ctx context.Context, email string, password string, ip string) (*dto.LoginResult, error) {
	var roles []string
	key := fmt.Sprintf("ratelimit:%s", ip)
	banDuration := service.config.Bruteforce.BanDuration
	// 1. Fetch user by email
	slog.Debug("Starting Request to DB")
	user, err := utils.SafeQuery(ctx, service.breaker, "Auth_service.Login", "", func() (*models.User, error) {
		user, role, err := service.repository.Login(ctx, service.db, email)
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return nil, domain.NewInvalidCredential(err)
		}
		// fmt.Println(err)
		roles = role
		return user, err
	})

	slog.Debug("Starting record strike to redis")
	if err != nil {
		var appErr *domain.AppError
		if errors.As(err, &appErr); errors.Is(appErr.Err, domain.ErrInvalidCredentials) {
			service.recordStrike(ctx, key, banDuration)
			return nil, err
		}
		return nil, err
	}

	// 2. Cryptographic comparison
	slog.Debug("Starting comparison password hash")
	if !utils.CheckPasswordHash(user.Password_hash, password) {
		service.recordStrike(ctx, key, banDuration)
		slog.Debug("Invalid credentials error in Login occured", "Error", "Password mismatch")
		// If the hash doesn't match, return a generic unauthorized error
		return nil, domain.NewInvalidCredential(fmt.Errorf("Password mismatch"))
	}

	// SUCCESS: Reset strikes so they have 5 fresh attempts next time
	service.redisClient.Del(ctx, key)

	// // 3. Check if account is active
	// if !user.IsActive {
	//     return "", domain.ErrAccountDisabled
	// }

	expirationTime := time.Now().Add(service.config.Security.AccessTokenTTL)

	// 4. Generate JWT (The "Passport")
	slog.Debug("Starting JWT generation")
	expiredAt, token, err := utils.GenerateJWT(user.ID, roles, expirationTime)
	if err != nil {
		return nil, domain.NewInternalError(fmt.Errorf("Auth_service.Login AccessToken generation failed: %w ", err))
	}

	userPublic := user.ToPublic()

	slog.Debug("Starting refresh token creation")
	refreshToken, err := service.tokenService.CreateSession(ctx, "", user.ID)
	if err != nil {
		// Architect's Choice: If session creation fails, we still have the user.
		// But for consistency, we treat this as a failure so the user can try logging in.
		return nil, domain.NewInternalError(fmt.Errorf("Auth_service.Login RefreshToken generation failed: %w ", err))
	}

	return &dto.LoginResult{
		AccessToken:  token,
		RefreshToken: refreshToken,
		Expiry:       expiredAt,
		User:         userPublic,
		Roles:        roles,
	}, nil
}

// The function where the user register
// After register complete, the user is automaticaly connected
// return *model.user, roles, accessToken, refreshToken, expiredAt, error
func (service *authService) Register(ctx context.Context, req dto.CreateUserRequest) (*dto.RegisterResult, error) {
	// 1. Hash the password (using Bcrypt)
	slog.Debug("Starting password hash")
	hashedPassword, err := utils.HashPassword(req.Password, service.config.Security.BcryptCost)
	if err != nil {
		return nil, domain.NewInternalError(fmt.Errorf("Auth_service.Register password hashing failed: %w ", err))
	}

	// 2. Prepare the Domain Model
	user := models.NewUser(req.Name, req.Email, string(hashedPassword))

	// 3. Define the default role for new users
	defaultRoles := []string{"user"}

	// 4. Execute via Repository Transaction
	slog.Debug("Starting DB transaction")
	err = dbconfig.WithTransaction(service.db, ctx, func(tx *sql.Tx) error {
		return utils.SafeExec(ctx, service.breaker, "Auth_service.Register", func() error {
			// Capture the result in a temp variable first
			repoErr := service.repository.CreateWithRoles(ctx, tx, user, defaultRoles)
			if repoErr != nil {
				if errors.Is(repoErr, domain.ErrDuplicateEmail) {
					return domain.NewDuplicateEmailError(repoErr)
				}
				if errors.Is(repoErr, domain.ErrInvalidInput) {
					return domain.NewInvalidInputError(repoErr)
				}
				return repoErr
			}
			return nil
		})
	})

	if err != nil {
		// Handle specific errors like "Duplicate Email"
		return nil, err
	}

	expirationTime := time.Now().Add(service.config.Security.AccessTokenTTL)

	// 5. Generate JWT (The "Passport")
	slog.Debug("Starting JWT generation")
	expiredAt, token, err := utils.GenerateJWT(user.ID, defaultRoles, expirationTime)
	if err != nil {
		// utils.SendErrors(w, r, http.StatusInternalServerError, "Token generation failed", err)
		return nil, domain.NewInternalError(fmt.Errorf("Auth_service.Register token generation failed: %w ", err))
	}

	// 6. We create a refresh token so the user stays logged in
	slog.Debug("Starting refresh token generation")
	refreshToken, err := service.tokenService.CreateSession(ctx, "", user.ID)
	if err != nil {
		// Architect's Choice: If session creation fails, we still have the user.
		// But for consistency, we treat this as a failure so the user can try logging in.
		return nil, err
	}

	return &dto.RegisterResult{
		User:         user,
		Roles:        defaultRoles,
		AccessToken:  token,
		RefreshToken: refreshToken,
		Expiry:       expiredAt,
	}, nil
}
