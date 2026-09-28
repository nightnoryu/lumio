package domain

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalid      = errors.New("invalid input")
	ErrUnauthorized = errors.New("invalid credentials or expired session")
	ErrInvitation   = errors.New("invalid or expired invitation")
	ErrConflict     = errors.New("email or subdomain already in use")
	ErrNotFound     = errors.New("not found")
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
}
type Session struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}
type Site struct {
	ID        string
	UserID    string
	Slug      string
	CreatedAt time.Time
}

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func NormalizeSlug(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if !slugPattern.MatchString(value) || strings.HasPrefix(value, "xn--") {
		return "", ErrInvalid
	}
	switch value {
	case "app", "api", "www", "admin", "grafana", "s3", "mail", "smtp", "support", "status", "static", "assets", "cdn", "lumio", "localhost":
		return "", ErrInvalid
	}
	return value, nil
}

func NormalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || len(value) > 254 {
		return "", ErrInvalid
	}
	return value, nil
}

func ValidatePassword(value string) error {
	if len(value) < 12 || len(value) > 128 {
		return ErrInvalid
	}
	return nil
}
