package auth

import (
	"log"
	"os"

	"github.com/golang-jwt/jwt"
)

func LoadKeys(privateKeyPath, publicKeyPath string) (signingKey interface{}, validationKey *TokenValidator) {
	privateBytes, readPrivateKeyError := os.ReadFile(privateKeyPath)

	if readPrivateKeyError != nil {
		log.Fatalf("Fatal: could not read private key file '%s': %v", privateKeyPath, readPrivateKeyError)
	}

	signingKey, parsePrivateKeyError := jwt.ParseECPrivateKeyFromPEM(privateBytes)

	if parsePrivateKeyError != nil {
		log.Fatalf("Fatal: could not parse private key from PEM: %v", parsePrivateKeyError)
	}

	validationKey, parsePrivateKeyError = NewTokenValidator(publicKeyPath)
	if parsePrivateKeyError != nil {
		log.Fatalf("Fatal: could not initialize token validator: %v", parsePrivateKeyError)
	}

	return
}