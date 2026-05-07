package domain

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidKey  = errors.New("invalid key")
	ErrInvalidName = errors.New("invalid name")
)

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func validateKey(key string) error {
	if !keyPattern.MatchString(strings.TrimSpace(key)) {
		return ErrInvalidKey
	}

	return nil
}

func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrInvalidName
	}

	return nil
}
