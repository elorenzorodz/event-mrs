package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt"
)

type TokenGenerator interface {
	Generate(email string) (string, error)
}

type TokenServiceImpl struct {
	SigningKey interface{}
}

func NewTokenGenerator(signingKey interface{}) TokenGenerator {
	return &TokenServiceImpl{
		SigningKey: signingKey,
	}
}

func (tokenServiceImpl *TokenServiceImpl) Generate(email string) (string, error) {
	if tokenServiceImpl.SigningKey == nil {
		return "", errors.New("JWT signing key is not configured")
	}

	newAccessToken := jwt.NewWithClaims(
		jwt.SigningMethodES256,
		jwt.MapClaims{
			"email": email,
			// Token expires in 1 hour
			"exp": time.Now().Add(time.Hour * 1).Unix(),
		})

	signedToken, signedTokenError := newAccessToken.SignedString(tokenServiceImpl.SigningKey)

	if signedTokenError != nil {
		return "", signedTokenError
	}

	return signedToken, nil
}