package common

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"unicode"

	"github.com/golang-jwt/jwt"
)

func IsEmailValid(email string) bool {
	emailRegex, emailValidationError := regexp.MatchString(`^[A-Za-z0-9]+([._\-][A-Za-z0-9]+)*@[A-Za-z0-9]+([\-\.][A-Za-z0-9]+)*\.[A-Za-z]{2,15}$`, email)

	if emailValidationError != nil {
		fmt.Printf("Invalid email: %s", emailValidationError)
	}

	return emailRegex
}

func IsPasswordValid(password string) bool {
	if len(password) < 12 || len(password) > 20 {
		return false
	}

	var hasUpper, hasLower, hasDigit, hasSpace bool

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsSpace(char):
			hasSpace = true
		}
	}

	return hasUpper && hasLower && hasDigit && !hasSpace
}

func ValidateJWTAndGetEmailClaim(signedToken string) (string, error) {
	publicBytes, publicKeyError := os.ReadFile("public.pem")

	if publicKeyError != nil {
		fmt.Printf("Read public key error: %s", publicKeyError)

		return "", fmt.Errorf("read public key error: %s", publicKeyError)
	}

	publicKey, publicKeyParseError := jwt.ParseECPublicKeyFromPEM(publicBytes)

	if publicKeyParseError != nil {
		fmt.Printf("Parse public key error: %s", publicKeyParseError)

		return "", fmt.Errorf("parse public key error: %s", publicKeyParseError)
	}

	parsedToken, parsedTokenError := jwt.Parse(signedToken, func(token *jwt.Token) (interface{}, error) {
		_, ok := token.Method.(*jwt.SigningMethodECDSA)
		
		if !ok {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Header["alg"])
		}

		return publicKey, nil
	})

	if parsedTokenError != nil {
		fmt.Printf("Token parse error: %s", parsedTokenError)

		return "", fmt.Errorf("token parse error: %s", parsedTokenError)
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok && parsedToken.Valid {
		emailClaim, exists := claims["email"]

		if !exists {
			fmt.Println("Email claim not found")

			return "", errors.New("invalid token")
		}

		email, ok := emailClaim.(string)

		if !ok {
			fmt.Println("Email claim is not a string")

			return "", errors.New("invalid token")
		}

		return email, nil
	}

	return "", errors.New("invalid token")
}