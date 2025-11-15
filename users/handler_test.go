package users_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elorenzorodz/event-mrs/users"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MockUserService struct {
	TestingType            *testing.T
	RegisterFunc func(ctx context.Context, req users.RegisterRequest) (*users.User, error)
	LoginFunc    func(ctx context.Context, req users.LoginRequest) (*users.UserAuthorized, error)
}

func (mockUserService *MockUserService) Register(ctx context.Context, req users.RegisterRequest) (*users.User, error) {
	if mockUserService.RegisterFunc == nil {
		mockUserService.TestingType.Fatal("Register was called, but RegisterFunc was not set.")
	}

	return mockUserService.RegisterFunc(ctx, req)
}

func (mockUserService *MockUserService) Login(ctx context.Context, req users.LoginRequest) (*users.UserAuthorized, error) {
	if mockUserService.LoginFunc == nil {
		mockUserService.TestingType.Fatal("Login was called, but LoginFunc was not set.")
	}

	return mockUserService.LoginFunc(ctx, req)
}

func setupTestRouter(service users.UserService) (*gin.Engine, *httptest.ResponseRecorder) {
	// Set Gin to test mode to suppress debug output
	gin.SetMode(gin.TestMode)

	// Create a response recorder (where the handler writes its response)
	recorder := httptest.NewRecorder()
	router := gin.New()

	// Initialize the handler's config with the mock service.
	apiConfig := users.UserAPIConfig{Service: service}

	// Setup the routes for the handlers we are testing.
	router.POST("/register", apiConfig.RegisterUser)
	router.POST("/login", apiConfig.LoginUser)

	return router, recorder
}

func TestRegisterUser(t *testing.T) {
	// Sample success response data
	testUserID := uuid.New().String()
	testTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	testUserFromService := &users.User{
		ID:        testUserID,
		FirstName: "Arthur",
		LastName:  "Morgan",
		Email:     "arthur.morgan@test.com",
		CreatedAt: testTime,
	}
	expectedUserJSON := map[string]interface{}{
		"id":         testUserID,
		"firstname":  "Arthur",
		"lastname":   "Morgan",
		"email":      "arthur.morgan@test.com",
		"created_at": testTime.Format(time.RFC3339Nano), 
	}
	
	tests := []struct {
		name                 string
		requestBody          gin.H 
		setupMock            func(mockService *MockUserService)
		expectedStatus       int
		expectedResponseBody gin.H 
	}{
		{
			name: "Success_StatusCreated",
			requestBody: gin.H{
				"firstname": "Arthur",
				"lastname":  "Morgan",
				"email":     "arthur.morgan@test.com",
				"password":  "GoodManArthur36!",
			},
			setupMock: func(mockService *MockUserService) {
				mockService.RegisterFunc = func(ctx context.Context, req users.RegisterRequest) (*users.User, error) {
					return testUserFromService, nil
				}
			},
			expectedStatus: http.StatusCreated,
			expectedResponseBody: gin.H{
				"user": expectedUserJSON,
			},
		},
		{
			name: "Failure_InvalidJSON",
			requestBody: nil,
			setupMock: func(mockService *MockUserService) {
				mockService.RegisterFunc = func(ctx context.Context, req users.RegisterRequest) (*users.User, error) {
					t.Fatal("Service Register should not be called on binding error")
					return nil, nil
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: gin.H{
				"error": "error parsing JSON, please check all required fields are present",
			},
		},
		{
			name: "Failure_WeakPassword_StatusBadRequest",
			requestBody: gin.H{
				"firstname": "Arthur",
				"lastname":  "Morgan",
				"email":     "arthur.morgan@test.com",
				"password":  "weak",
			},
			setupMock: func(mockService *MockUserService) {
				mockService.RegisterFunc = func(ctx context.Context, req users.RegisterRequest) (*users.User, error) {
					return nil, users.ErrPasswordWeak
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: gin.H{
				"error": users.ErrPasswordWeak.Error(),
			},
		},
		{
			name: "Failure_EmailExists_StatusConflict",
			requestBody: gin.H{
				"firstname": "Arthur",
				"lastname":  "Morgan",
				"email":     "arthur.morgan@test.com",
				"password":  "Password12345!",
			},
			setupMock: func(mockService *MockUserService) {
				mockService.RegisterFunc = func(ctx context.Context, req users.RegisterRequest) (*users.User, error) {
					return nil, users.ErrEmailExists
				}
			},
			expectedStatus: http.StatusConflict,
			expectedResponseBody: gin.H{
				"error": "email address already registered",
			},
		},
		{
			name: "Failure_InvalidEmail_StatusBadRequest",
			requestBody: gin.H{
				"firstname": "Arthur",
				"lastname":  "Morgan",
				"email":     "invalid@testcom",
				"password":  "Password12345!",
			},
			setupMock: func(mockService *MockUserService) {
				mockService.RegisterFunc = func(ctx context.Context, req users.RegisterRequest) (*users.User, error) {
					return nil, users.ErrPasswordInvalid
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: gin.H{
				"error": "invalid email format or missing fields",
			},
		},
		{
			name: "Failure_InternalServerError",
			requestBody: gin.H{
				"firstname": "Arthur",
				"lastname":  "Morgan",
				"email":     "arthur.morgan@test.com",
				"password":  "Password12345!",
			},
			setupMock: func(mockService *MockUserService) {
				mockService.RegisterFunc = func(ctx context.Context, req users.RegisterRequest) (*users.User, error) {
					return nil, errors.New("database connection failed")
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedResponseBody: gin.H{
				"error": "failed to register user due to an internal error",
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockService := &MockUserService{TestingType: t}
			testCase.setupMock(mockService)
			router, recorder := setupTestRouter(mockService)

			var reqBody bytes.Buffer

			if testCase.requestBody != nil {
				json.NewEncoder(&reqBody).Encode(testCase.requestBody)
			} else {
				reqBody.WriteString("this is not json")
			}

			req, _ := http.NewRequest(http.MethodPost, "/register", &reqBody)
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(recorder, req)

			// Assertions
			if recorder.Code != testCase.expectedStatus {
				t.Fatalf("Expected status %d, got %d. Response: %s", testCase.expectedStatus, recorder.Code, recorder.Body.String())
			}

			var actualResponse gin.H

			if err := json.Unmarshal(recorder.Body.Bytes(), &actualResponse); err != nil {
				t.Fatalf("Could not unmarshal response body: %v. Body: %s", err, recorder.Body.String())
			}

			// Special handling for the success case due to the time field's format.
			if testCase.name == "Success_StatusCreated" {
				userMap, ok := actualResponse["user"].(map[string]interface{})

				if !ok {
					t.Fatalf("Expected response to contain 'user' map, got: %v", actualResponse)
				}

				// Assert on non-time fields
				expectedUserMap := testCase.expectedResponseBody["user"].(map[string]interface{})
				
				if expectedUserMap["id"] != userMap["id"] {
					t.Errorf("ID mismatch. Expected %s, got %s", expectedUserMap["id"], userMap["id"])
				}
				if expectedUserMap["firstname"] != userMap["firstname"] {
					t.Errorf("FirstName mismatch. Expected %s, got %s", expectedUserMap["firstname"], userMap["firstname"])
				}
				if expectedUserMap["lastname"] != userMap["lastname"] {
					t.Errorf("LastName mismatch. Expected %s, got %s", expectedUserMap["lastname"], userMap["lastname"])
				}
				if expectedUserMap["email"] != userMap["email"] {
					t.Errorf("Email mismatch. Expected %s, got %s", expectedUserMap["email"], userMap["email"])
				}
				
				// Optional: More robust time assertion (requires parsing).
				if _, ok := userMap["created_at"].(string); !ok {
					t.Errorf("CreatedAt field is not a string, got %T", userMap["created_at"])
				}

			} else {
				// General assertion for error cases.
				if fmt.Sprintf("%v", actualResponse) != fmt.Sprintf("%v", testCase.expectedResponseBody) {
					t.Errorf("Response body mismatch. \nExpected: %v\nActual: %v", testCase.expectedResponseBody, actualResponse)
				}
			}
		})
	}
}

func TestLoginUser(t *testing.T) {
	// Sample success response data.
	expectedAuthResponse := users.UserAuthorized{
		Email:       "arthur.morgan@login.com",
		AccessToken: "mocked_jwt_token_12345",
	}

	tests := []struct {
		name                 string
		requestBody          gin.H
		setupMock            func(mockService *MockUserService)
		expectedStatus       int
		expectedResponseBody gin.H
	}{
		{
			name: "Success_StatusOK",
			requestBody: gin.H{
				"email":    "arthur.morgan@login.com",
				"password": "ValidPassword1!",
			},
			setupMock: func(mockService *MockUserService) {
				mockService.LoginFunc = func(ctx context.Context, req users.LoginRequest) (*users.UserAuthorized, error) {
					return &expectedAuthResponse, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedResponseBody: gin.H{
				"user": gin.H{
					"email": "arthur.morgan@login.com",
					"access_token": "mocked_jwt_token_12345",
				},
			},
		},
		{
			name: "Failure_InvalidJSON",
			requestBody: nil,
			setupMock: func(mockService *MockUserService) {
				mockService.LoginFunc = func(ctx context.Context, req users.LoginRequest) (*users.UserAuthorized, error) {
					t.Fatal("Service Login should not be called on binding error")
					return nil, nil
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedResponseBody: gin.H{
				"error": "error parsing JSON, please check all required fields are present",
			},
		},
		{
			name: "Failure_InvalidCredentials_StatusUnauthorized",
			requestBody: gin.H{
				"email":    "arthur.morgan@login.com",
				"password": "WrongPassword",
			},
			setupMock: func(mockService *MockUserService) {
				mockService.LoginFunc = func(ctx context.Context, req users.LoginRequest) (*users.UserAuthorized, error) {
					return nil, users.ErrPasswordInvalid
				}
			},
			expectedStatus: http.StatusUnauthorized,
			expectedResponseBody: gin.H{
				"error": "invalid email or password",
			},
		},
		{
			name: "Failure_InternalServerError",
			requestBody: gin.H{
				"email":    "arthur.morgan@login.com",
				"password": "ValidPassword1!",
			},
			setupMock: func(mockService *MockUserService) {
				mockService.LoginFunc = func(ctx context.Context, req users.LoginRequest) (*users.UserAuthorized, error) {
					return nil, errors.New("token generation failed")
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedResponseBody: gin.H{
				"error": "failed to login, please try again in a few minutes",
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockService := &MockUserService{TestingType: t}
			testCase.setupMock(mockService)
			router, recorder := setupTestRouter(mockService)

			var reqBody bytes.Buffer

			if testCase.requestBody != nil {
				json.NewEncoder(&reqBody).Encode(testCase.requestBody)
			} else {
				reqBody.WriteString("this is not json")
			}

			req, _ := http.NewRequest(http.MethodPost, "/login", &reqBody)
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, req)

			// Assertions
			if recorder.Code != testCase.expectedStatus {
				t.Fatalf("Expected status %d, got %d. Response: %s", testCase.expectedStatus, recorder.Code, recorder.Body.String())
			}

			var actualResponse gin.H

			if err := json.Unmarshal(recorder.Body.Bytes(), &actualResponse); err != nil {
				t.Fatalf("Could not unmarshal response body: %v. Body: %s", err, recorder.Body.String())
			}

			if fmt.Sprintf("%v", actualResponse) != fmt.Sprintf("%v", testCase.expectedResponseBody) {
				t.Errorf("Response body mismatch. \nExpected: %v\nActual: %v", testCase.expectedResponseBody, actualResponse)
			}
		})
	}
}