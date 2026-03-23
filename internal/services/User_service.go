package services

import (
	"context"
	"database/sql"
	"example/hello/internal/dbconfig"
	"example/hello/internal/domain"
	"example/hello/internal/dto"
	"example/hello/internal/models"
	"example/hello/internal/repositories"
	"example/hello/internal/utils"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
	"golang.org/x/sync/errgroup"
)

// Define the Interface (The Contract)
type UserService interface {
	GetUsersLists(ctx context.Context, page int, pageSize int) ([]*models.User, *dto.PaginationMetadata, error)
	GetUsersByID(ctx context.Context, id int) (*models.User, error)
	CreateUser(ctx context.Context, user dto.CreateUserRequest) (int, error)
	ReplaceUser(ctx context.Context, id int, updateData dto.PutUserRequest) (*models.User, error)
	UpdateProfile(ctx context.Context, id int, updateData dto.PatchUserRequest) (*models.User, error)
	SoftDeleteUser(ctx context.Context, id int) error
	GetUsersListsWithCursor(ctx context.Context, cursor string, limit int) ([]*models.User, *dto.CursorPaginationMetadata, error)
}

// Define the Struct (The Implementation)
type userService struct {
	db         *sql.DB
	breaker    *gobreaker.CircuitBreaker
	repository repositories.UserRepository
	redis      *redis.Client
}

// The Constructor (How to create the service)
func NewUserService(db *sql.DB, b *gobreaker.CircuitBreaker, repo repositories.UserRepository, rdb *redis.Client) UserService {
	return &userService{
		db:         db,
		repository: repo,
		breaker:    b,
		redis:      rdb,
	}
}

// Helper to prevent blocking the main request
func (service *userService) updateCountCache(key string, count int) {
	// Create a dedicated background context with a 5-second limit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set TTL to 10 minutes - Architect's balance between freshness and speed
	service.redis.Set(ctx, key, count, 10*time.Minute)
}

// GetUsersLists fetches all users from the DB with pagination
func (service *userService) GetUsersLists(ctx context.Context, page int, pageSize int) ([]*models.User, *dto.PaginationMetadata, error) {
	// Defense: Cap the limit
	if pageSize > 100 {
		pageSize = 20
	}
	if pageSize <= 0 {
		pageSize = 10 // Default
	}

	// var
	users := make([]*models.User, 0, pageSize)
	totalRecords := 0
	cacheKey := "users:total_count"

	// Logic for Offset
	offset := utils.CalculateOffset(page, pageSize)

	// Step One: Get the Count (Try Cache First)
	// We do this BEFORE starting the DB transaction
	cachedCount, redisErr := service.redis.Get(ctx, cacheKey).Int()
	if redisErr == nil {
		slog.Debug("Check redis cache for user list count", "value", cachedCount)
		totalRecords = cachedCount
	}

	// Start the Snapshotted Transaction
	err := dbconfig.WithTransactionReadOnly(service.db, ctx, func(tx *sql.Tx) error {
		// Use errgroup for Context-Aware Parallelism
		g, gCtx := errgroup.WithContext(ctx)

		// Task 1: Fetch Users
		g.Go(func() error {
			var err error
			// Use the group context (gCtx) so if one fails, others stop
			users, err = utils.SafeQuery(gCtx, service.breaker, "service.GetUsersLists (GetAllUsers)", "No users found", func() ([]*models.User, error) {
				return service.repository.GetAllUsers(gCtx, tx, pageSize, offset)
			})
			return err
		})

		// Task 2: Fetch Count
		if redisErr != nil {
			g.Go(func() error {
				count, err := service.repository.GetTotalUserCount(gCtx, tx)
				if err == nil {
					slog.Debug("Updating count user list cache in redis")
					totalRecords = count
					// Async: Update cache so we don't block the response
					go service.updateCountCache(cacheKey, count)
				}
				return err
			})
		}

		// Wait for both to finish
		return g.Wait()
	})

	if err != nil {
		slog.Debug("GetUsersLists error triggered", "ErrorValue", err)
		return nil, nil, err
	}

	// Exception: Always allow Page 1, even if the DB is empty.
	if totalRecords > 0 && offset >= totalRecords {
		slog.Debug("GetUsersLists error triggered", "ErrorValue", err)
		return nil, nil, domain.NewNotFoundError(fmt.Errorf("No user found"), "No user found")
	}

	// Metadata Calculation
	metadata := &dto.PaginationMetadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		TotalRecords: totalRecords,
		TotalPages:   utils.CalculateTotalPages(totalRecords, pageSize),
	}

	return users, metadata, nil
}

// Get Users by ID
func (service *userService) GetUsersByID(ctx context.Context, id int) (*models.User, error) {
	return utils.SafeQuery(ctx, service.breaker, "GetUsersByID", "User not found", func() (*models.User, error) {
		return service.repository.GetUsersByID(ctx, service.db, id)
	})
}

// Create user
func (service *userService) CreateUser(ctx context.Context, req dto.CreateUserRequest) (int, error) {
	hashedPassword, err := utils.HashPassword(req.Password, 12)
	if err != nil {
		return 0, domain.NewInternalError(err)
	}

	// Prepare the Domain Model
	user := models.NewUser(req.Name, req.Email, string(hashedPassword))

	err = utils.SafeExec(ctx, service.breaker, "service.CreateUser", func() error {
		// Capture the result in a temp variable first
		repoErr := service.repository.CreateUser(ctx, service.db, user)
		return repoErr
	})

	if err != nil {
		return 0, err
	}

	return user.ID, nil
}

// Update user
func (service *userService) ReplaceUser(ctx context.Context, id int, updateData dto.PutUserRequest) (*models.User, error) {
	var updatedUser *models.User

	// Fetch current data from Repo
	existingUser, errorUserId := utils.SafeQuery(ctx, service.breaker, "service.ReplaceUser (GetUsersByID)", "User not found",
		func() (*models.User, error) {
			return service.repository.GetUsersByID(ctx, service.db, id)
		})

	if errorUserId != nil {
		return nil, errorUserId
	}

	// Domain Logic: Update all data
	existingUser.PutUpdateFields(updateData.Name, updateData.Email)

	err := utils.SafeExec(ctx, service.breaker, "service.ReplaceUser (UpdateUser)", func() error {
		// Capture the result in a temp variable first
		res, repoErr := service.repository.UpdateUser(ctx, service.db, id, existingUser)
		if repoErr == nil {
			updatedUser = res // Only assign to the return variable if it actually succeeded
		}
		return repoErr
	})

	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

// Patch user only updates provided fields
func (service *userService) UpdateProfile(ctx context.Context, id int, updateData dto.PatchUserRequest) (*models.User, error) {
	var updatedUser *models.User

	// Fetch current data from Repo
	existingUser, errorUserId := utils.SafeQuery(ctx, service.breaker, "service.UpdateProfile (GetUsersByID)", "User not found",
		func() (*models.User, error) {
			return service.repository.GetUsersByID(ctx, service.db, id)
		})

	if errorUserId != nil {
		return nil, errorUserId
	}

	// Domain Logic: Update all data
	existingUser.PatchUpdateFields(updateData.Name, updateData.Email)

	err := utils.SafeExec(ctx, service.breaker, "service.UpdateProfile (UpdateUser)", func() error {
		// Capture the result in a temp variable first
		res, repoErr := service.repository.UpdateUser(ctx, service.db, id, existingUser)
		if repoErr == nil {
			updatedUser = res // Only assign to the return variable if it actually succeeded
		}
		return repoErr
	})

	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

// Soft delete user
func (service *userService) SoftDeleteUser(ctx context.Context, id int) error {
	return utils.SafeExec(ctx, service.breaker, "service.SoftDeleteUser", func() error {
		return service.repository.SoftDeleteUser(ctx, service.db, id)
	})
}

// GetUsersLists fetches all users from the DB with Cursor-based pagination
func (service *userService) GetUsersListsWithCursor(ctx context.Context, cursor string, limit int) ([]*models.User, *dto.CursorPaginationMetadata, error) {
	// Decoding the cursor
	lastCreatedAt, lastID, err := utils.DecodeCursor(cursor)
	if err != nil {
		return nil, nil, domain.NewInvalidInputError(fmt.Errorf("Invalid pagination cursor"))
	}

	// Defense: Cap the limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// var
	users := make([]*models.User, 0, limit)
	totalRecords := 0
	cacheKey := "users:total_count"

	// Step One: Get the Count (Try Cache First)
	// We do this BEFORE starting the DB transaction
	cachedCount, redisErr := service.redis.Get(ctx, cacheKey).Int()

	// Start the Snapshotted Transaction
	err = dbconfig.WithTransactionReadOnly(service.db, ctx, func(tx *sql.Tx) error {
		// Use errgroup for Context-Aware Parallelism
		g, gCtx := errgroup.WithContext(ctx)

		// Task 1: Fetch Users
		g.Go(func() error {
			var err error
			// Use the group context (gCtx) so if one fails, others stop
			users, err = utils.SafeQuery(gCtx, service.breaker, "service.GetUsersListsWithCursor (GetAllUsersWithCursor)", "No users found", func() ([]*models.User, error) {
				return service.repository.GetAllUsersWithCursor(gCtx, tx, lastCreatedAt, lastID, limit)
			})
			return err
		})

		// Task 2: Fetch Count
		if redisErr != nil {
			slog.Debug("Check redis cache for user list count", "value", cachedCount)
			g.Go(func() error {
				count, err := service.repository.GetTotalUserCount(gCtx, tx)
				if err == nil {
					slog.Debug("Updating count user list cache in redis")
					totalRecords = count
					// Async: Update cache so we don't block the response
					go service.updateCountCache(cacheKey, count)
				}
				return err
			})
		} else {
			totalRecords = cachedCount
		}

		// Wait for both to finish
		return g.Wait()
	})

	if err != nil {
		slog.Debug("GetUsersLists error triggered", "ErrorValue", err)
		return nil, nil, err
	}

	// If there are NO users
	if len(users) == 0 {
		return nil, nil, domain.NewNotFoundError(fmt.Errorf("no users in database"), "No users found")
	}

	// If we are on Page 1 and the list is empty, but count > 0,
	// that means the specific cursor filtered everyone out.
	if lastCreatedAt.IsZero() && len(users) == 0 {
		return nil, nil, domain.NewNotFoundError(fmt.Errorf("no users found"), "User list is empty")
	}

	// Establishing next cursor
	nextCursor := ""
	if len(users) > 0 {
		lastUser := users[len(users)-1]
		nextCursor = utils.EncodeCursor(lastUser.CreatedAt, int64(lastUser.ID))
	}

	// Metadata Calculation
	metadata := &dto.CursorPaginationMetadata{
		NextCursor:   nextCursor,
		HasMore:      len(users) == limit,
		TotalRecords: totalRecords,
		TotalPages:   utils.CalculateTotalPages(totalRecords, limit),
	}

	return users, metadata, nil
}
