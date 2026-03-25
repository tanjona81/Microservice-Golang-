package handlers

import (
	"encoding/json"
	"example/hello/internal/client"
	"example/hello/internal/config"
	"example/hello/internal/contextkeys"
	"example/hello/internal/dto"
	"example/hello/internal/services"
	"example/hello/internal/utils"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	userv1 "github.com/tanjona81/gRPC-Golang-/gen/go"
)

type UserHandler struct {
	service   services.UserService
	appConfig *config.Config
}

func NewUserHandler(cfg *config.Config, s services.UserService) *UserHandler {
	return &UserHandler{
		appConfig: cfg,
		service:   s,
	}
}

// Get all users
func (handle *UserHandler) GetUsersOffsetHandler(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Request passed succesfully")
	// Extract query params and convert strings to ints
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	ctx := r.Context()

	// Call the persistence layer
	responses, metadata, err := handle.service.GetUsersLists(ctx, page, size)

	// Error 500
	if err != nil {
		utils.HandleError(w, r, err)
		return
	}

	utils.SendSuccessWithMetadata(w, http.StatusOK, responses, metadata)
}

// Get all users
func (handle *UserHandler) GetUsersCursorHandler(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Request passed succesfully")
	// Extract query params and convert strings to ints
	cursor := r.URL.Query().Get("cursor")
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	ctx := r.Context()

	// Call the persistence layer
	responses, metadata, err := handle.service.GetUsersListsWithCursor(ctx, cursor, size)

	// Error 500
	if err != nil {
		utils.HandleError(w, r, err)
		return
	}

	utils.SendSuccessWithMetadata(w, http.StatusOK, responses, metadata)
}

// Get users by ID
func (handle *UserHandler) GetUsersByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Getting the ID from the context
	id := ctx.Value(contextkeys.UserIDKey).(int)

	// Call the persistence layer
	users, err := handle.service.GetUsersByID(ctx, id)

	// Error handling
	if err != nil {
		utils.HandleError(w, r, err)
		return
	}
	utils.SendSuccess(w, http.StatusOK, users)
}

// Create user
func (handle *UserHandler) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Decode JSON
	user, err := utils.JSONDecoder[dto.CreateUserRequest](w, r)
	if err != nil {
		return
	}

	// Cleaning up the request body
	defer r.Body.Close()

	// Validate Input
	if err := utils.ValidateStruct(user); err != nil {
		utils.HandleError(w, r, err)
		return
	}

	// Calling the service to save the user
	userID, errCreation := handle.service.CreateUser(ctx, user)
	if errCreation != nil {
		utils.HandleError(w, r, errCreation)
		return
	}

	// Set the Location header so the client knows where the new resource is
	w.Header().Set("Location", fmt.Sprintf("api/v1/users/%d", userID))

	// Use your SendSuccess helper
	utils.SendSuccess(w, http.StatusCreated, map[string]int{"id": userID})
}

// Update user
func (handle *UserHandler) PutUpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	// Variable
	var user dto.PutUserRequest
	ctx := r.Context()

	// Getting the ID from the context
	id := ctx.Value(contextkeys.UserIDKey).(int)

	// Decoding the JSON body into the struct 'user'
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		utils.SendSuccess(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON format"})
		return
	}

	// Cleaning up the request body
	defer r.Body.Close()

	// Calling the service to save the user
	userUpdated, errorRepository := handle.service.ReplaceUser(ctx, id, user)
	if errorRepository != nil {
		fmt.Println(errorRepository)
		utils.SendSuccess(w, http.StatusInternalServerError, map[string]string{"error": "Could not update the user"})
		return
	}

	utils.SendSuccess(w, http.StatusOK, userUpdated)
}

// Patch Update user
func (handle *UserHandler) PatchUpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	var user dto.PatchUserRequest
	ctx := r.Context()

	// Getting the ID from the context
	id := ctx.Value(contextkeys.UserIDKey).(int)

	// Decoding the JSON body into the struct 'user'
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		utils.SendSuccess(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON format"})
		return
	}

	// Cleaning up the request body
	defer r.Body.Close()

	// Calling the service to save the user
	userUpdated, errorRepository := handle.service.UpdateProfile(ctx, id, user)
	if errorRepository != nil {
		fmt.Println(errorRepository)
		utils.SendSuccess(w, http.StatusInternalServerError, map[string]string{"error": "Could not update the user"})
		return
	}

	utils.SendSuccess(w, http.StatusOK, userUpdated)
}

// Update user
func (handle *UserHandler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Getting the ID from the context
	id := ctx.Value(contextkeys.UserIDKey).(int)

	// Calling the service to delete the user
	errorRepository := handle.service.SoftDeleteUser(ctx, id)
	if errorRepository != nil {
		utils.SendSuccess(w, http.StatusInternalServerError, map[string]string{"error": "Could not delete the user"})
		return
	}

	utils.SendSuccess(w, http.StatusNoContent, nil)
}

// Get all users
func (handle *UserHandler) GetUserFromGRPC(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Getting the ID from the context
	id := ctx.Value(contextkeys.UserIDKey).(int)

	// Setup the "Phone"
	// userClient, conn, err := client.NewUserClient("grpc-user-service.go-grpc:50051")
	userClient, conn, err := client.NewUserClient(handle.appConfig.Grpc)
	defer conn.Close()

	// Error handling
	if err != nil {
		utils.HandleError(w, r, err)
		return
	}

	// The Call: Map the 'id' to the gRPC request
	resp, err := userClient.GetUser(ctx, &userv1.GetUserRequest{
		UserId: strconv.Itoa(id),
	})

	// 4. Handle gRPC Errors (Timeout, Not Found, etc.)
	if err != nil {
		utils.HandleError(w, r, err)
		return
	}
	utils.SendSuccess(w, http.StatusOK, resp)
}
