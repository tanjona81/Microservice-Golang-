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
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
)

// Define the Interface (The Contract)
type TokenService interface {
	CreateSession(ctx context.Context, oldHash string, userID int) (string, error)
	RotateSession(ctx context.Context, oldRawToken string) (*dto.TokenRotationResult, error)
	RevokeSession(ctx context.Context, rawToken string) error
}

// Define the Struct (The Implementation)
type tokenService struct {
	db             *sql.DB
	breaker        *gobreaker.CircuitBreaker
	repository     repositories.TokenRepository
	userRepository repositories.UserRepository
	authRepository repositories.AuthRepository
	config         *config.Config
	redis          *redis.Client
}

// The Constructor (How to create the service)
func NewTokenService(
	db *sql.DB,
	b *gobreaker.CircuitBreaker,
	repo repositories.TokenRepository,
	userRepo repositories.UserRepository,
	authRepo repositories.AuthRepository,
	c *config.Config,
) TokenService {
	if b == nil {
		log.Fatal("AuthService requires a non-nil CircuitBreaker")
	}
	return &tokenService{
		db:             db,
		breaker:        b,
		repository:     repo,
		userRepository: userRepo,
		authRepository: authRepo,
		config:         c,
	}
}

func (service *tokenService) CreateSession(ctx context.Context, oldRawToken string, userID int) (string, error) {
	// Generate the raw token (The "Secret")
	rawToken, err := utils.GenerateRefreshToken()
	if err != nil {
		return "", domain.NewInternalError(fmt.Errorf("CreateSession refreshToken generation failed: %w ", err))
	}

	// Hash it for the database (The "Lock")
	hashedToken := utils.HashToken(rawToken)

	// We only hash the old token if it's provided (not a fresh login)
	var oldHashedToken string
	if oldRawToken != "" {
		oldHashedToken = utils.HashToken(oldRawToken)
	}

	expiresAt := time.Now().Add(service.config.Security.RefreshTokenTTL)

	// Store the hash in the DB
	err = dbconfig.WithTransaction(service.db, ctx, func(tx *sql.Tx) error {
		return utils.SafeExec(ctx, service.breaker, "service.CreateSession", func() error {
			err := service.repository.RefreshSession(ctx, tx, oldHashedToken, hashedToken, userID, expiresAt)
			if err != nil {
				if errors.Is(err, domain.ErrCompromisedSession) {
					slog.Warn("[WARNING] SECURITY ALERT: Token reuse detected",
						"user_id", userID,
						"action", "revoke_all_sessions",
					)
					return domain.NewCompromisedSession(err)
				}
				return err
			}
			return err
		})
	})

	if err != nil {
		return "", err
	}

	// Return the RAW token to be sent to the user in a cookie
	return rawToken, nil
}

func (service *tokenService) RotateSession(ctx context.Context, oldRawToken string) (*dto.TokenRotationResult, error) {
	user := &models.UserPublic{}
	var roleString string
	var roles []string

	// Hash the old token to find it in DB
	if oldRawToken == "" {
		return nil, domain.NewUnauthorizedError(errors.New("invalid session"))
	}
	oldHash := utils.HashToken(oldRawToken)

	// Generate NEW pair
	newRawToken, err := utils.GenerateRefreshToken()
	if err != nil {
		return nil, domain.NewInternalError(err)
	}
	newHash := utils.HashToken(newRawToken)

	expiresAt := time.Now().Add(service.config.Security.RefreshTokenTTL)

	// DATABASE TRANSACTION: Delete old, Insert new (Rotation)
	err = dbconfig.WithTransaction(service.db, ctx, func(tx *sql.Tx) error {
		return utils.SafeExec(ctx, service.breaker, "service.CreateSession", func() error {
			err = service.repository.RefreshAndFetchUser(ctx, tx, oldHash, newHash, expiresAt, user, &roleString)
			if err != nil {
				if errors.Is(err, domain.ErrCompromisedSession) {
					slog.Warn("[WARNING] SECURITY ALERT: Refresh token reuse detected",
						"token_hash", oldHash,
						"action", "revoke_all_sessions",
					)
					return domain.NewCompromisedSession(err)
				}
				return err
			}
			return err
		})
	})

	if err != nil {
		return nil, err
	}

	expirationTime := time.Now().Add(service.config.Security.AccessTokenTTL)

	// Only split if we actually have data
	// This prevents the [""] slice issue.
	if roleString != "" {
		roles = strings.Split(roleString, ",")
	}

	// data cleaning
	// If the DB has spaces (e.g., "admin, user"), trim them.
	for i := range roles {
		roles[i] = strings.TrimSpace(roles[i])
	}

	// Generate new JWT Access Token
	expiredAt, accessToken, err := utils.GenerateJWT(user.ID, roles, expirationTime)
	if err != nil {
		// utils.SendErrors(w, r, http.StatusInternalServerError, "Token generation failed", err)
		return nil, domain.NewInternalError(fmt.Errorf("token generation failed: %w ", err))
	}

	return &dto.TokenRotationResult{
		AccessToken:  accessToken,
		RefreshToken: newRawToken,
		Expiry:       expiredAt,
		User:         user,
		Roles:        roles,
	}, nil
}

func (service *tokenService) RevokeSession(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		slog.Debug("RevokeSession called with empty token; skipping.")
		return nil
	}

	tokenHash := utils.HashToken(rawToken)

	err := utils.SafeExec(ctx, service.breaker, "service.RevokeSession", func() error {
		return service.repository.DeleteRefreshToken(ctx, service.db, tokenHash)
	})

	if err != nil {
		// If the token is already gone, logout is still "successful"
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}

	return nil
}
