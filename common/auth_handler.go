package common

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	bytes, hashPasswordError := bcrypt.GenerateFromPassword([]byte(password), 14)

	return string(bytes), hashPasswordError
}