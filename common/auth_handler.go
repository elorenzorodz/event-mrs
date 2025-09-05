package common

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, hashPasswordError := bcrypt.GenerateFromPassword([]byte(password), 14)

	return string(bytes), hashPasswordError
}

func VerifyPassword(password, hashedPassword string) error {
    return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func GetJWT(authorizationHeader string) (string, error) {
	if authorizationHeader == "" {
		return "", errors.New("authorization header is required")
	}

	// Split authorization header into Bearer and token.
	bearerTokenParts := strings.Split(authorizationHeader, " ")

	if len(bearerTokenParts) != 2 {
		return "", errors.New("invalid authorization header format")
	}

	if bearerTokenParts[0] != "Bearer" {
		return "", errors.New("invalid authorization scheme, expected 'Bearer'")
	}

	return bearerTokenParts[1], nil
}