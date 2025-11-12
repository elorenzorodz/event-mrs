package users_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elorenzorodz/event-mrs/config"
	"github.com/elorenzorodz/event-mrs/internal/database"
	"github.com/elorenzorodz/event-mrs/users"
	"github.com/google/uuid"
)

type MockDBQueries struct {
	*config.BaseMock
	tTesting           *testing.T
	CreateUserFunc     func(ctx context.Context, arg database.CreateUserParams) (database.User, error)
	GetUserByEmailFunc func(ctx context.Context, email string) (database.User, error)
}

func (mockDBQueries *MockDBQueries) CreateUser(ctx context.Context, arg database.CreateUserParams) (database.User, error) {
	if mockDBQueries.CreateUserFunc == nil {
		mockDBQueries.tTesting.Fatalf("CreateUser was called, but no expectation (CreateUserFunc) was set.")
	}

	return mockDBQueries.CreateUserFunc(ctx, arg)
}

func (mockDBQueries *MockDBQueries) GetUserByEmail(ctx context.Context, email string) (database.User, error) {
	if mockDBQueries.GetUserByEmailFunc == nil {
		mockDBQueries.tTesting.Fatalf("GetUserByEmail was called, but no expectation (GetUserByEmailFunc) was set.")
	}

	return mockDBQueries.GetUserByEmailFunc(ctx, email)
}

type MockTokenGenerator struct {
	tTesting     *testing.T
	GenerateFunc func(email string) (string, error)
}

func (m *MockTokenGenerator) Generate(email string) (string, error) {
	if m.GenerateFunc == nil {
		m.tTesting.Fatalf("Generate was called, but no expectation (GenerateFunc) was set.")
	}

	return m.GenerateFunc(email)
}

// assertNoError asserts that the error is nil.
func assertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: expected no error, got: %v", msg, err)
	}
}

// assertError asserts that the error is non-nil and matches the expected error message.
func assertError(t *testing.T, expected, actual error, msg string) {
	t.Helper()
	if actual == nil || actual.Error() != expected.Error() {
		t.Fatalf("%s: expected error %v, got: %v", msg, expected, actual)
	}
}

func TestRegister(tTesting *testing.T) {
	ctx := context.Background()


	testUserDB := database.User{
		ID:        uuid.New(),
		Firstname: "Arthur",
		Lastname:  "Morgan",
		Email:     "arthur.morgan@test.com",
		Password:  "GoodManArthur36!",
		CreatedAt: time.Now(),
	}

	tests := []struct {
		name            string
		registerRequest users.RegisterRequest
		setupMocks      func(mockDB *MockDBQueries, mockTokenGen *MockTokenGenerator)
		expectedError   error
	}{
		{
			name: "Success",
			registerRequest: users.RegisterRequest{
				FirstName: "Arthur",
				LastName:  "Morgan",
				Email:     "arthur.morgan@test.com",
				Password:  "GoodManArthur36!",
			},
			setupMocks: func(mockDB *MockDBQueries, _ *MockTokenGenerator) {
				mockDB.CreateUserFunc = func(ctx context.Context, arg database.CreateUserParams) (database.User, error) {
					return testUserDB, nil
				}
			},
			expectedError: nil,
		},
		{
			name: "Failure_WeakPassword",
			registerRequest: users.RegisterRequest{
				Email:    "arthur.morgan@test.com",
				Password: "weak",
			},
			setupMocks:    func(_ *MockDBQueries, _ *MockTokenGenerator) {},
			expectedError: users.ErrPasswordWeak,
		},
		{
			name: "Failure_EmailExists",
			registerRequest: users.RegisterRequest{
				Email:    "arthur.morgan@test.com",
				Password: "Password12345!",
			},
			setupMocks: func(mockDB *MockDBQueries, _ *MockTokenGenerator) {
				mockDB.CreateUserFunc = func(ctx context.Context, arg database.CreateUserParams) (database.User, error) {
					return database.User{}, errors.New("db unique constraint error")
				}
			},
			expectedError: users.ErrEmailExists,
		},
		{
			name: "Failure_InvalidEmail",
			registerRequest: users.RegisterRequest{
				Email:    "invalid@testcom",
				Password: "Password12345!",
			},
			setupMocks:    func(_ *MockDBQueries, _ *MockTokenGenerator) {},
			expectedError: users.ErrPasswordInvalid,
		},
	}

	for _, tc := range tests {
		tTesting.Run(tc.name, func(t *testing.T) {
			mockDB := &MockDBQueries{tTesting: t}
			mockTokenGen := &MockTokenGenerator{tTesting: t}
			service := users.NewService(mockDB, mockTokenGen)

			tc.setupMocks(mockDB, mockTokenGen)

			user, err := service.Register(ctx, tc.registerRequest)

			// Assertions.
			if tc.expectedError != nil {
				assertError(t, tc.expectedError, err, "Register error assertion")

				if user != nil {
					t.Fatalf("Expected nil user, got %v", user)
				}
			} else {
				assertNoError(t, err, "Register success assertion")

				if user == nil {
					t.Fatal("Expected non-nil user")
				}
				// Only assert on non-sensitive fields
				if user.FirstName != tc.registerRequest.FirstName {
					t.Errorf("Expected user first name %s, got %s", tc.registerRequest.FirstName, user.FirstName)
				}
			}
		})
	}
}

func TestLogin(tTesting *testing.T) {
	ctx := context.Background()

	testHashedPassword := "$2a$14$10WUQj2cgEhGkX9uF.aqnOGpX7sk4v5gbY2RaooNoLm90hzCRnWmC"

	validEmail := "arthur.morgan@login.com"
	validPassword := "ValidPassword1!"

	testUserDB := database.User{
		Email:    validEmail,
		Password: testHashedPassword,
	}

	tests := []struct {
		name          string
		loginRequest  users.LoginRequest
		setupMocks    func(mockDB *MockDBQueries, mockTokenGen *MockTokenGenerator)
		expectedError error
	}{
		{
			name: "Success",
			loginRequest: users.LoginRequest{
				Email:    validEmail,
				Password: validPassword,
			},
			setupMocks: func(mockDB *MockDBQueries, mockTokenGen *MockTokenGenerator) {
				mockDB.GetUserByEmailFunc = func(ctx context.Context, email string) (database.User, error) {
					return testUserDB, nil
				}
				mockTokenGen.GenerateFunc = func(email string) (string, error) {
					return "mocked_token", nil
				}
			},
			expectedError: nil,
		},
		{
			name: "Failure_UserNotFound",
			loginRequest: users.LoginRequest{
				Email:    "notfound@test.com",
				Password: validPassword,
			},
			setupMocks: func(mockDB *MockDBQueries, _ *MockTokenGenerator) {
				mockDB.GetUserByEmailFunc = func(ctx context.Context, email string) (database.User, error) {
					return database.User{}, errors.New("sql: no rows in result set")
				}
			},
			expectedError: users.ErrPasswordInvalid,
		},
		{
			name: "Failure_WrongPassword",
			loginRequest: users.LoginRequest{
				Email:    validEmail,
				Password: "WrongPassword",
			},
			setupMocks: func(mockDB *MockDBQueries, _ *MockTokenGenerator) {
				mockDB.GetUserByEmailFunc = func(ctx context.Context, email string) (database.User, error) {
					return testUserDB, nil
				}
			},
			expectedError: users.ErrPasswordInvalid,
		},
		{
			name: "Failure_TokenGenerationError",
			loginRequest: users.LoginRequest{
				Email:    validEmail,
				Password: validPassword,
			},
			setupMocks: func(mockDB *MockDBQueries, mockTokenGen *MockTokenGenerator) {
				mockDB.GetUserByEmailFunc = func(ctx context.Context, email string) (database.User, error) {
					return testUserDB, nil
				}
				mockTokenGen.GenerateFunc = func(email string) (string, error) {
					return "", errors.New("token failure")
				}
			},
			expectedError: errors.New("internal token generation error"),
		},
	}

	for _, tc := range tests {
		tTesting.Run(tc.name, func(t *testing.T) {
			mockDB := &MockDBQueries{tTesting: t}
			mockTokenGen := &MockTokenGenerator{tTesting: t}
			service := users.NewService(mockDB, mockTokenGen)

			tc.setupMocks(mockDB, mockTokenGen)

			userAuth, err := service.Login(ctx, tc.loginRequest)

			if tc.expectedError != nil {
				assertError(t, tc.expectedError, err, "Login error assertion")

				if userAuth != nil {
					t.Fatalf("Expected nil userAuth, got %v", userAuth)
				}
			} else {
				assertNoError(t, err, "Login success assertion")

				if userAuth == nil {
					t.Fatal("Expected non-nil userAuth")
				}

				if userAuth.AccessToken != "mocked_token" {
					t.Errorf("Expected token 'mocked_token', got %s", userAuth.AccessToken)
				}
			}
		})
	}
}