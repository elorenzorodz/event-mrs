package validation

import (
	"regexp"
	"unicode"
)

func IsEmailValid(email string) bool {
	emailRegex := `^[A-Za-z0-9]+([._\-\][A-Za-z0-9]+)*@[A-Za-z0-9]+([\-\.][A-Za-z0-9]+)*\.[A-Za-z]{2,15}$`
	
	isMatch, err := regexp.MatchString(emailRegex, email)

	if err != nil {
		return false
	}

	return isMatch
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

	// Must have upper, lower, digit, and NO spaces.
	return hasUpper && hasLower && hasDigit && !hasSpace
}