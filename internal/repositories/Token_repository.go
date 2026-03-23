package repositories

import (
	"context"
	"database/sql"
	"errors"
	"example/hello/internal/domain"
	"example/hello/internal/models"
	"fmt"
	"time"
)

// Define the Interface (The Contract)
type TokenRepository interface {
	RefreshSession(ctx context.Context, db domain.DBTX, oldHash string, newHash string, userID int, expiredAt time.Time) error
	DeleteRefreshToken(ctx context.Context, db domain.DBTX, tokenHash string) error
	RefreshAndFetchUser(ctx context.Context, db domain.DBTX, oldHash string, newHash string, expiresAt time.Time, user *models.UserPublic, roleString *string) error
}

// Define the Struct (The Implementation)
type tokenRepository struct {
	db *sql.DB
}

// The Constructor (How to create the repo)
func NewTokenRepository(db *sql.DB) TokenRepository {
	return &tokenRepository{db: db}
}

func (repo *tokenRepository) RefreshSession(ctx context.Context, db domain.DBTX, oldHash string, newHash string, userID int, expiresAt time.Time) error {
	// Delete/Revoke the old token
	// Can add more layer of security
	// (if oldHash is provided but the rows affected by the delete query is 0 -> alert user or revoke all tokens for the user)
	if oldHash != "" {
		res, err := db.ExecContext(ctx, "DELETE FROM t_users_refresh_tokens WHERE token_hash = ? AND id_user = ?",
			oldHash, userID)
		if err != nil {
			return domain.WrapRepositoryError(err, false, fmt.Errorf("tokenRepository.RefreshSession (Delete): %w", err))
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			// SECURITY ALERT: This token was either already used or never existed.
			// This is a sign of a "Replay Attack".
			// Recommendation: Revoke ALL tokens for this userID for safety.
			_, _ = db.ExecContext(ctx, "DELETE FROM t_users_refresh_tokens WHERE id_user = ?", userID)
			return domain.ErrCompromisedSession
		}
	}

	// Delete this user's old expired tokens to keep the table lean
	_, _ = db.ExecContext(ctx, "DELETE FROM t_users_refresh_tokens WHERE id_user = ? AND expires_at < NOW()", userID)

	// Insert the new token
	query := "INSERT INTO t_users_refresh_tokens (id_user, token_hash, expires_at) VALUES (?, ?, ?)"
	_, err := db.ExecContext(ctx, query, userID, newHash, expiresAt)
	if err != nil {
		return domain.WrapRepositoryError(err, false, fmt.Errorf("tokenRepository.RefreshSession (Insert): %w", err))
	}
	return err
}

func (repo *tokenRepository) DeleteRefreshToken(ctx context.Context, db domain.DBTX, tokenHash string) error {
	query := `DELETE FROM t_users_refresh_tokens WHERE token_hash = ?`

	res, err := db.ExecContext(ctx, query, tokenHash)
	if err != nil {
		return domain.WrapRepositoryError(err, false, fmt.Errorf("tokenRepository.DeleteRefreshToken: %w", err))
	}

	// Validate that something actually happened
	rows, err := res.RowsAffected()
	if err != nil {
		return domain.WrapRepositoryError(err, false, fmt.Errorf("tokenRepository.DeleteRefreshToken.RowsAffected: %w", err))
	}

	if rows == 0 {
		// We return ErrNotFound so the Service layer can decide if this
		// is a problem or just a redundant logout.
		return domain.ErrNotFound
	}

	return nil
}

func (repo *tokenRepository) RefreshAndFetchUser(ctx context.Context, db domain.DBTX, oldHash string,
	newHash string, expiresAt time.Time, user *models.UserPublic, roleString *string) error {
	// Fetch the user + his roles
	userQuery := `SELECT u.id, u.name, u.email, IFNULL(GROUP_CONCAT(r.slug), '') as roles
						FROM t_users u
						LEFT JOIN t_user_roles ur ON u.id = ur.user_id
						LEFT JOIN t_roles r ON ur.role_id = r.id
						JOIN t_users_refresh_tokens urt ON u.id = urt.id_user
						WHERE urt.token_hash = ?
						GROUP BY u.id;`
	err := db.QueryRowContext(ctx, userQuery, oldHash).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		roleString,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Potential Replay Attack: Delete all sessions for safety
			// We'd need the userID, which we don't have here.
			// This is why some architects keep a log or pass userID from Service.
			return domain.ErrCompromisedSession
		}
		return domain.WrapRepositoryError(err, false, fmt.Errorf("tokenRepository.RefreshAndFetchUser: %w", err))
	}

	// Delete/Revoke the old token
	// Can add more layer of security
	// (if oldHash is provided but the rows affected by the delete query is 0 -> alert user or revoke all tokens for the user)
	if oldHash != "" {
		res, errDelete := db.ExecContext(ctx, "DELETE FROM t_users_refresh_tokens WHERE token_hash = ? AND id_user = ?",
			oldHash, user.ID)
		if errDelete != nil {
			return domain.WrapRepositoryError(errDelete, false, fmt.Errorf("tokenRepository.RefreshSession (Delete): %w", errDelete))
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			// SECURITY ALERT: This token was either already used or never existed.
			// This is a sign of a "Replay Attack".
			// Recommendation: Revoke ALL tokens for this userID for safety.
			_, _ = db.ExecContext(ctx, "DELETE FROM t_users_refresh_tokens WHERE id_user = ?", user.ID)
			return domain.ErrCompromisedSession
		}
	}

	// Delete this user's old expired tokens to keep the table lean
	_, _ = db.ExecContext(ctx, "DELETE FROM t_users_refresh_tokens WHERE id_user = ? AND expires_at < NOW()", user.ID)

	// Insert the new token
	query := "INSERT INTO t_users_refresh_tokens (id_user, token_hash, expires_at) VALUES (?, ?, ?)"
	_, err = db.ExecContext(ctx, query, user.ID, newHash, expiresAt)
	if err != nil {
		return domain.WrapRepositoryError(err, false, fmt.Errorf("tokenRepository.RefreshAndFetchUser (Insert): %w", err))
	}

	return err
}
