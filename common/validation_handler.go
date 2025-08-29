package common

import (
	"fmt"
	"regexp"
	"unicode"
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