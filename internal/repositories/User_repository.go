package repositories

import (
	"context"
	"database/sql"
	"example/hello/internal/domain"
	"example/hello/internal/models"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Define the Interface (The Contract)
type UserRepository interface {
	GetAllUsers(ctx context.Context, db domain.DBTX, limit, offset int) ([]*models.User, error)
	GetUsersByID(ctx context.Context, db domain.DBTX, id int) (*models.User, error)
	CreateUser(ctx context.Context, db domain.DBTX, user *models.User) error
	UpdateUser(ctx context.Context, db domain.DBTX, id int, existingUser *models.User) (*models.User, error)
	SoftDeleteUser(ctx context.Context, db domain.DBTX, id int) error
	GetTotalUserCount(ctx context.Context, db domain.DBTX) (int, error)
	GetRoles(ctx context.Context, db domain.DBTX, id_user int) ([]string, error)
	GetAllUsersWithCursor(ctx context.Context, db domain.DBTX, lastCreatedAt time.Time, lastID int64, limit int) ([]*models.User, error)
}

// Define the Struct (The Implementation)
type userRepository struct {
	db *sql.DB
}

// The Constructor (How to create the repo)
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func isDuplicateEntryError(err error) bool {
	// 1062 is the MySQL error code for Duplicate Entry
	if mysqlErr, ok := err.(*mysql.MySQLError); ok {
		return mysqlErr.Number == 1062
	}
	return false
}

// GetAllUsers fetches all users from the DB using offset
func (repo *userRepository) GetAllUsers(ctx context.Context, db domain.DBTX, limit int, offset int) ([]*models.User, error) {
	query := `
        SELECT id, name, email, created_at, updated_at 
        FROM t_users 
        WHERE deleted_at IS NULL
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?`

	// Use QueryContext to handle timeouts/cancellations
	rows, err := db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		slog.Debug("GetAllUsers query error triggered")
		return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("repository.GetAllUsers query: %w", err))
	}
	defer rows.Close()

	// Pre-allocate slice with capacity to avoid re-allocations
	users := make([]*models.User, 0, limit)

	for rows.Next() {
		user := &models.User{}
		err = rows.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			slog.Debug("GetAllUsers scan error triggered")
			return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("repository.GetAllUsers scan: %w", err))
		}
		users = append(users, user)
	}

	// Final check for errors during iteration
	if err = rows.Err(); err != nil {
		slog.Debug("GetAllUsers iteration error triggered")
		return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("repository.GetAllUsers iteration: %w", err))
	}

	slog.Debug("GetAllUsers check error triggered", "ErrorValue", err)

	return users, nil
}

// Get Users by ID
func (repo *userRepository) GetUsersByID(ctx context.Context, db domain.DBTX, id int) (*models.User, error) {

	// Initializing query
	user := &models.User{}
	query := "SELECT id, name, email, created_at, updated_at FROM t_users WHERE id = ? AND deleted_at IS NULL"

	// Executing query
	err := db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	// Handling error
	if err != nil {
		return nil, domain.WrapRepositoryError(err, true, fmt.Errorf("userRepository.GetUsersByID (id=%d): %w", id, err))
	}

	return user, nil
}

// Create user
func (repo *userRepository) CreateUser(ctx context.Context, db domain.DBTX, user *models.User) error {
	query := `INSERT INTO t_users (name, email, password_hash) VALUES (?, ?, ?)`
	res, err := db.ExecContext(ctx, query, user.Name, user.Email, user.Password_hash)
	if err != nil {
		if isDuplicateEntryError(err) {
			return domain.ErrConflict // You'll need to define this in your domain
		}
		return domain.WrapRepositoryError(err, false, fmt.Errorf("userRepository.CreateUser: %w", err))
	}
	// Get the ID of the user we just created
	userID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("userRepository.CreateUser: %w", err)
	}

	// Set the ID back to the user object for the response
	user.ID = int(userID)
	return nil
}

// Update user
func (repo *userRepository) UpdateUser(ctx context.Context, db domain.DBTX, id int, existingUser *models.User) (*models.User, error) {
	query := "UPDATE t_users SET name = ?, email = ? WHERE id = ? AND deleted_at IS NULL"

	// Execute the update
	result, err := db.ExecContext(ctx, query, existingUser.Name, existingUser.Email, id)
	if err != nil {
		return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("userRepository.CreateUser: %w", err))
	}

	// Check if the user actually existed
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("userRepository.UpdateUser.RowsAffected: %w", err))
	}
	if rows == 0 {
		return nil, domain.ErrNotFound
	}

	return existingUser, nil
}

// Soft delete user
func (repo *userRepository) SoftDeleteUser(ctx context.Context, db domain.DBTX, id int) error {
	query := `UPDATE t_users SET deleted_at = NOW(), CONCAT(email, '-deleted-', UNIX_TIMESTAMP()) 
				WHERE id = ? AND deleted_at IS NULL`

	// We pass the ID as the last argument to match the WHERE clause
	res, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return domain.WrapRepositoryError(err, false, fmt.Errorf("userRepository.SoftDeleteUser: %w", err))
	}
	// Check if the user was already deleted or didn't exist
	rows, err := res.RowsAffected()
	if err != nil {
		return domain.WrapRepositoryError(err, false, fmt.Errorf("userRepository.SoftDeleteUser.RowsAffected: %w", err))
	}

	if rows == 0 {
		return domain.ErrNotFound
	}
	return err
}

// Fetch the total count of the user in the DB that is not deleted
func (repo *userRepository) GetTotalUserCount(ctx context.Context, db domain.DBTX) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM t_users WHERE deleted_at IS NULL"
	err := db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, domain.WrapRepositoryError(err, false, fmt.Errorf("userRepository.GetTotalUserCount: %w", err))
	}
	return count, nil
}

// Fetch the roles of a user
func (repo *userRepository) GetRoles(ctx context.Context, db domain.DBTX, id_user int) ([]string, error) {
	var roles []string

	query := `
        SELECT r.slug
        FROM t_users u
        LEFT JOIN t_user_roles ur ON u.id = ur.user_id
        LEFT JOIN t_roles r ON ur.role_id = r.id
        WHERE u.id = ? AND u.deleted_at IS NULL`

	rows, err := db.QueryContext(ctx, query, id_user)
	if err != nil {
		return nil, domain.WrapRepositoryError(err, true, fmt.Errorf("userRepository.GetRoles: %w", err))
	}
	defer rows.Close()

	for rows.Next() {
		var roleSlug sql.NullString
		err := rows.Scan(&roleSlug)
		if err != nil {
			return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("userRepository.GetRoles scan error: %w", err))
		}
		if roleSlug.Valid {
			roles = append(roles, roleSlug.String)
		}
	}

	// Check for errors that happened during iteration (Standard practice)
	if err = rows.Err(); err != nil {
		return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("userRepository.GetRoles iteration error: %w", err))
	}

	return roles, nil
}

// GetAllUsersWithCursor fetches all users from the DB using Cursor-based Pagination
func (repo *userRepository) GetAllUsersWithCursor(ctx context.Context, db domain.DBTX, lastCreatedAt time.Time, lastID int64, limit int) ([]*models.User, error) {
	// Start with the base query
	query := `SELECT id, name, email, created_at, updated_at FROM t_users WHERE deleted_at IS NULL`
	args := []any{}

	// If a cursor is provided, add the "Seek" logic
	slog.Debug("Check lastcreatedat.IsZero()", "value", lastCreatedAt.IsZero())
	if !lastCreatedAt.IsZero() {
		slog.Debug("GetAllUsersWithCursor: Check if inside !lastcreatedat.IsZero()", "value", "we are")
		query += ` AND (created_at, id) < (?, ?)`
		args = append(args, lastCreatedAt, lastID)
	}

	// Always finish with the sort and limit
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	// Use QueryContext to handle timeouts/cancellations
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		slog.Debug("GetAllUsersWithCursor query error triggered")
		return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("repository.GetAllUsersWithCursor query: %w", err))
	}
	defer rows.Close()

	// Pre-allocate slice with capacity to avoid re-allocations
	users := make([]*models.User, 0, limit)

	for rows.Next() {
		user := &models.User{}
		err = rows.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			slog.Debug("GetAllUsersWithCursor scan error triggered")
			return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("repository.GetAllUsersWithCursor scan: %w", err))
		}
		users = append(users, user)
	}

	// Final check for errors during iteration
	if err = rows.Err(); err != nil {
		slog.Debug("GetAllUsersWithCursor iteration error triggered")
		return nil, domain.WrapRepositoryError(err, false, fmt.Errorf("repository.GetAllUsersWithCursor iteration: %w", err))
	}

	slog.Debug("GetAllUsersWithCursor check error triggered", "ErrorValue", err)

	return users, nil
}
