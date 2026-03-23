package handlers

// import (
// 	"context"
// 	"example/hello/contextkeys"
// 	"example/hello/domain"
// 	"example/hello/dto"
// 	"example/hello/models"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"
// 	"time"
// )

// // MockUserService fulfills the UserService interface
// type MockUserService struct {
// 	// We only create a mockFn for the specific method we are testing
// 	// to keep the test code clean.
// 	GetUsersByIDFn func(ctx context.Context, id int) (models.User, error)
// 	// CreateUserFn     func(user models.User) error
// 	// GetUsersListsFn  func(page, pageSize int) (dto.UserListResponse, error)
// 	// SoftDeleteUserFn func(id int) error
// }

// // 1. Implementation of GetUsersByID (Matches interface exactly)
// func (m *MockUserService) GetUsersByID(ctx context.Context, id int) (models.User, error) {
// 	if m.GetUsersByIDFn != nil {
// 		return m.GetUsersByIDFn(ctx, id)
// 	}
// 	return models.User{}, nil
// }

// // 2. Implementation of CreateUser
// func (m *MockUserService) CreateUser(ctx context.Context, user dto.CreateUserRequest) error {
// 	return nil
// }

// // 3. Implementation of GetUsersLists
// func (m *MockUserService) GetUsersLists(ctx context.Context, page int, pageSize int) ([]models.User, dto.PaginationMetadata, error) {
// 	// FIX: You cannot return nil for a struct. Return empty struct instead.
// 	return nil, dto.PaginationMetadata{}, nil
// }

// // 4. Implementation of PutUpdateUser
// func (m *MockUserService) PutUpdateUser(ctx context.Context, id int, updateData dto.PutUserRequest) (models.User, error) {
// 	return models.User{}, nil
// }

// // 5. Implementation of PatchUser
// func (m *MockUserService) PatchUser(ctx context.Context, id int, updateData dto.PatchUserRequest) (models.User, error) {
// 	return models.User{}, nil
// }

// // 6. Implementation of SoftDeleteUser
// func (m *MockUserService) SoftDeleteUser(ctx context.Context, id int) error {
// 	return nil
// }

// func (m *MockUserService) Login(ctx context.Context, email string, password string) (models.User, []string, string, time.Time, error) {
// 	return models.User{}, nil, "", time.Time{}, nil
// }

// func TestGetUserHandler_TableDriven(t *testing.T) {
// 	// 1. Define the "Table" of test cases
// 	tests := []struct {
// 		name           string
// 		userID         int
// 		mockBehavior   func(ctx context.Context, id int) (models.User, error)
// 		expectedStatus int
// 	}{
// 		{
// 			name:   "Success Case",
// 			userID: 1,
// 			mockBehavior: func(ctx context.Context, id int) (models.User, error) {
// 				return models.User{ID: 1, Name: "Senior Dev"}, nil
// 			},
// 			expectedStatus: http.StatusOK,
// 		},
// 		{
// 			name:   "User Not Found",
// 			userID: 99,
// 			mockBehavior: func(ctx context.Context, id int) (models.User, error) {
// 				return models.User{}, domain.NewNotFoundError(domain.ErrNotFound, "User not found")
// 			},
// 			expectedStatus: http.StatusNotFound,
// 		},
// 	}

// 	// 2. Loop through the table
// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			// Setup Mock
// 			mockSvc := &MockUserService{
// 				GetUsersByIDFn: tc.mockBehavior,
// 			}
// 			hdl := NewUserHandler(mockSvc)

// 			// Setup Request with Context
// 			req := httptest.NewRequest("GET", "/", nil)
// 			ctx := context.WithValue(req.Context(), contextkeys.RequestIDKey, tc.userID)
// 			req = req.WithContext(ctx)
// 			rr := httptest.NewRecorder()

// 			// Execute
// 			hdl.GetUsersByIDHandler(rr, req)

// 			// Assert
// 			if rr.Code != tc.expectedStatus {
// 				t.Errorf("%s: expected status %d, got %d", tc.name, tc.expectedStatus, rr.Code)
// 			}
// 		})
// 	}
// }

// // func TestGetUserHandler_Success(t *testing.T) {
// // 	// 1. Setup the Mock Behavior
// // 	mockSvc := &MockUserService{
// // 		GetUsersByIDFn: func(id int) (models.User, error) {
// // 			return models.User{ID: 10, Name: "Senior Dev"}, nil
// // 		},
// // 	}

// // 	// 2. Inject the Mock into the Handler
// // 	hdl := NewUserHandler(mockSvc)

// // 	// 3. Create a request and manually inject the ID into the Context
// // 	// (Simulating what our Middleware does in production)
// // 	req := httptest.NewRequest("GET", "/users/10", nil)
// // 	ctx := context.WithValue(req.Context(), middleware.UserIDKey, 10)
// // 	req = req.WithContext(ctx)

// // 	// 4. Create a Recorder to capture the output
// // 	rr := httptest.NewRecorder()

// // 	// 5. Execute the Handler
// // 	hdl.GetUsersByIDHandler(rr, req)

// // 	// 6. Assertions (The Verification)
// // 	// if rr.Code != http.StatusOK {
// // 	//     t.Errorf("expected status 200, got %d", rr.Code)
// // 	// }

// // 	// 6. Assertions
// // 	if rr.Code != http.StatusOK {
// // 		t.Errorf("expected status 200, got %d", rr.Code)
// // 	}

// // 	var responseUser models.User
// // 	if err := json.NewDecoder(rr.Body).Decode(&responseUser); err != nil {
// // 		t.Fatalf("failed to decode response: %v", err)
// // 	}

// // 	if responseUser.Name != "Senior Dev" {
// // 		t.Errorf("expected name 'Senior Dev', got '%s'", responseUser.Name)
// // 	}

// // 	// expectedBody := `{"ID":10,"Name":"Senior Dev"}` // Simplified for example
// // 	// Check if rr.Body.String() contains your expected JSON
// // }
