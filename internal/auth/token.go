package auth

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/golang-jwt/jwt"
)

var ErrInvalidToken = errors.New("invalid or expired token")

func GetTokenFromHeader(authorizationHeader string) (string, error) {
	if authorizationHeader == "" {
		return "", errors.New("authorization header is required")
	}

	// Split authorization header into Bearer and token.
	bearerTokenParts := strings.Split(authorizationHeader, " ")

	if len(bearerTokenParts) != 2 || bearerTokenParts[0] != "Bearer" {
		return "", errors.New("invalid authorization header format, expected 'Bearer <token>'")
	}

	return bearerTokenParts[1], nil
}

type TokenValidator struct {
	publicKey *ecdsa.PublicKey
}

func NewTokenValidator(publicKeyPath string) (*TokenValidator, error) {
	publicBytes, publicKeyError := os.ReadFile(publicKeyPath)

	if publicKeyError != nil {
		return nil, fmt.Errorf("read public key error: %w", publicKeyError)
	}

	publicKey, publicKeyParseError := jwt.ParseECPublicKeyFromPEM(publicBytes)

	if publicKeyParseError != nil {
		return nil, fmt.Errorf("parse public key error: %w", publicKeyParseError)
	}

	return &TokenValidator{publicKey: publicKey}, nil
}

func (tokenValidator *TokenValidator) ValidateAndGetEmailClaim(signedToken string) (string, error) {
	parsedToken, parsedTokenError := jwt.Parse(signedToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Header["alg"])
		}

		return tokenValidator.publicKey, nil
	})

	if parsedTokenError != nil || !parsedToken.Valid {
		return "", ErrInvalidToken
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	
	if !ok {
		return "", ErrInvalidToken
	}
	
	emailClaim, exists := claims["email"]

	if !exists {
		return "", ErrInvalidToken
	}

	email, ok := emailClaim.(string)

	if !ok {
		return "", ErrInvalidToken
	}

	return email, nil
}