package auth

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	if err := validatePassword("", password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(hash string, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func validatePassword(username string, password string) error {
	if len(password) < 10 {
		return errors.New("password must be at least 10 characters")
	}
	if strings.TrimSpace(password) == "" {
		return errors.New("password cannot be blank")
	}
	if username != "" && strings.EqualFold(username, password) {
		return errors.New("password cannot be the same as username")
	}
	return nil
}
