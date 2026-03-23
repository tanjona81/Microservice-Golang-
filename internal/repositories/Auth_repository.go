package repositories

import (
	"context"
	"database/sql"
	"errors"
	"example/hello/internal/domain"
	"example/hello/internal/models"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// Define the Interface (The Contract)
type AuthRepository interface {
	Login(ctx context.Context, db domain.DBTX, email string) (*models.User, []string, error)
	CreateWithRoles(ctx context.Context, db domain.DBTX, user *models.User, roles []string) error
	GetUserIDByAuth(ctx context.Context, db domain.DBTX, tokenHash string) (models.User, error)
}

// Define the Struct (The Implementation)
type authRepository struct {
	db *sql.DB
}

// The Constructor (How to create the repo)
func NewAuthRepository(db *sql.DB) AuthRepository {
	return &authRepository{db: db}
}

// placeholders returns a string like "?, ?, ?" for SQL IN clauses
func (repo *authRepository) placeholders(n int) string {
	ps := make([]string, n)
	for i := 0; i < n; i++ {
		ps[i] = "?"
	}
	return strings.Join(ps, ", ")
}

// convertToInterface converts a string slice to an interface slice for sql.Exec
func (repo *authRepository) convertToInterface(s []string) []interface{} {
	i := make([]interface{}, len(s))
	for k, v := range s {
		i[k] = v
	}
	return i
}

// Fetch the total count of the user in the DB that is not deleted
func (repo *authRepository) Login(ctx context.Context, db domain.DBTX, email string) (*models.User, []string, error) {
	user := &models.User{}
	var roleString string

	query := `SELECT u.id, u.name, u.email, u.password_hash, IFNULL(GROUP_CONCAT(r.slug), '') as roles
		FROM t_users u
		LEFT JOIN t_user_roles ur ON u.id = ur.user_id
		LEFT JOIN t_roles r ON ur.role_id = r.id
		WHERE u.email = ? AND u.deleted_at IS NULL
		GROUP BY u.id`

	err := db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password_hash,
		&roleString,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, domain.ErrInvalidCredentials
		}
		return nil, nil, domain.WrapRepositoryError(err, false, fmt.Errorf("AuthRepository.Login: %w", err))
	}

	var roles []string
	// Only split if we actually have data
	// This prevents the [""] slice issue.
	if roleString != "" {
		roles = strings.Split(roleString, ",")
		// data cleaning
		// If the DB has spaces (e.g., "admin, editor"), trim them.
		for i := range roles {
			roles[i] = strings.TrimSpace(roles[i])
		}
	}

	return user, roles, nil
}

func (repo *authRepository) CreateWithRoles(ctx context.Context, db domain.DBTX, user *models.User, roles []string) error {
	// Insert User
	query := `INSERT INTO t_users (name, email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, query, user.Name, user.Email, user.Password_hash, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		// Check if it's a MySQL driver error
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			// 1062 is the MySQL code for Duplicate Entry
			if mysqlErr.Number == 1062 {
				return domain.ErrDuplicateEmail
			}
		}
		return domain.WrapRepositoryError(err, false,
			fmt.Errorf("Auth_repository.CreateWithRoles failed to insert user: %w", err))
	}

	// Get the ID of the user we just created
	userID, err := res.LastInsertId()
	if err != nil {
		return domain.WrapRepositoryError(err, false,
			fmt.Errorf("Auth_repository.CreateWithRoles failed to use LastInsertId method: %w", err))
	}

	// Set the ID back to the user object for the response
	user.ID = int(userID)

	if len(roles) == 0 {
		return nil
	}

	// Optimized Role Mapping: Bulk Insert
	// Instead of a loop with SELECT + INSERT, we use a single query to insert mapping
	// by selecting IDs from t_roles directly into the insert statement.
	roleMappingQuery := `
			INSERT IGNORE INTO t_user_roles (user_id, role_id)
			SELECT ?, id FROM t_roles WHERE slug IN (` + repo.placeholders(len(roles)) + `)`

	// Prepare arguments: userID first, then all the slugs
	args := append([]interface{}{userID}, repo.convertToInterface(roles)...)

	res, err = db.ExecContext(ctx, roleMappingQuery, args...)
	if err != nil {
		return domain.WrapRepositoryError(err, false,
			fmt.Errorf("Auth_repository.CreateWithRoles insert t_user_roles: %w", err))
	}

	rows, _ := res.RowsAffected()
	if int(rows) != len(roles) {
		// This means one of the roles provided doesn't exist in t_roles
		return domain.ErrInvalidInput
	}

	return domain.WrapRepositoryError(err, false,
		fmt.Errorf("Auth_repository.CreateWithRoles: %w", err))
}

// Get Users by refresh token
func (repo *authRepository) GetUserIDByAuth(ctx context.Context, db domain.DBTX, tokenHash string) (models.User, error) {

	// Initializing query
	var user models.User
	query := `SELECT u.id, u.name, u.email, u.created_at, u.updated_at 
	FROM t_users u
	JOIN t_users_refresh_tokens t ON u.id = t.id_user
	WHERE token_hash = ? AND t.expires_at > NOW()`

	// Use QueryRowContext instead of QueryRow
	err := db.QueryRowContext(ctx, query, tokenHash).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	// Handling error
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, domain.ErrUnauthorized
		}
		// Use %w to wrap the error so the service layer knows what happened
		return models.User{}, fmt.Errorf("database error fetching user by id: %w", err)
	}

	return user, nil
}

// func (repo *authRepository) CreateWithRoles(ctx context.Context, db domain.DBTX, user *models.User, roles []string) error {
// 	// Start of the transaction
// 	return dbconfig.WithTransaction(db, ctx, func(tx *sql.Tx) error {
// 		// Insert User
// 		query := `INSERT INTO t_users (name, email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
// 		res, err := tx.ExecContext(ctx, query, user.Name, user.Email, user.Password_hash, user.CreatedAt, user.UpdatedAt)
// 		if err != nil {
// 			// Check if it's a MySQL driver error
// 			if mysqlErr, ok := err.(*mysql.MySQLError); ok {
// 				// 1062 is the MySQL code for Duplicate Entry
// 				if mysqlErr.Number == 1062 {
// 					return domain.ErrDuplicateEmail
// 				}
// 			}
// 			return domain.WrapRepositoryError(err, false,
// 				fmt.Errorf("Auth_repository.CreateWithRoles failed to insert user: %w", err))
// 		}

// 		// Get the ID of the user we just created
// 		userID, err := res.LastInsertId()
// 		if err != nil {
// 			return domain.WrapRepositoryError(err, false,
// 				fmt.Errorf("Auth_repository.CreateWithRoles failed to use LastInsertId method: %w", err))
// 		}

// 		// Set the ID back to the user object for the response
// 		user.ID = int(userID)

// 		if len(roles) == 0 {
// 			return nil
// 		}

// 		// Optimized Role Mapping: Bulk Insert
// 		// Instead of a loop with SELECT + INSERT, we use a single query to insert mapping
// 		// by selecting IDs from t_roles directly into the insert statement.
// 		roleMappingQuery := `
// 			INSERT IGNORE INTO t_user_roles (user_id, role_id)
// 			SELECT ?, id FROM t_roles WHERE slug IN (` + repo.placeholders(len(roles)) + `)`

// 		// Prepare arguments: userID first, then all the slugs
// 		args := append([]interface{}{userID}, repo.convertToInterface(roles)...)

// 		res, err = tx.ExecContext(ctx, roleMappingQuery, args...)
// 		if err != nil {
// 			return domain.WrapRepositoryError(err, false,
// 				fmt.Errorf("Auth_repository.CreateWithRoles insert t_user_roles: %w", err))
// 		}

// 		rows, _ := res.RowsAffected()
// 		if int(rows) != len(roles) {
// 			// This means one of the roles provided doesn't exist in t_roles
// 			return domain.ErrInvalidInput
// 		}

// 		return domain.WrapRepositoryError(err, false,
// 			fmt.Errorf("Auth_repository.CreateWithRoles: %w", err))
// 	})
// }
