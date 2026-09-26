package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"

	"lumio/internal/domain"
)

const SessionLifetime = 7 * 24 * time.Hour

type Store interface {
	Register(context.Context, domain.User, string, domain.Session) error
	UserByEmail(context.Context, string) (domain.User, error)
	CreateSession(context.Context, domain.Session, string) error
	SessionUser(context.Context, string) (domain.User, error)
	DeleteSession(context.Context, string) error
	CreateSite(context.Context, domain.Site) error
	Sites(context.Context, string) ([]domain.Site, error)
	Site(context.Context, string, string) (domain.Site, error)
	RenameSite(context.Context, string, string, string) (domain.Site, error)
	Invite(context.Context, string, string, time.Time) error
	IssueReset(context.Context, string, string, time.Time) error
	ResetPassword(context.Context, string, string) error
}

type Passwords interface {
	Hash(string) (string, error)
	Verify(string, string) bool
}

type Service struct {
	Store     Store
	Passwords Passwords
}

func Token() string { return rand.Text() }
func TokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func CSRF(token string) string { return TokenHash("csrf:" + token) }

func (s *Service) Register(ctx context.Context, email, password, invitation string) (string, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return "", err
	}
	if err = domain.ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := s.Passwords.Hash(password)
	if err != nil {
		return "", err
	}
	user := domain.User{ID: uuid.NewString(), Email: email, PasswordHash: hash}
	token, session := newSession(user.ID)
	if err = s.Store.Register(ctx, user, TokenHash(invitation), session); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil || domain.ValidatePassword(password) != nil {
		return "", domain.ErrUnauthorized
	}
	user, err := s.Store.UserByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}
	// Verify performs the same expensive derivation for unknown accounts.
	valid := s.Passwords.Verify(user.PasswordHash, password)
	if err != nil || !valid {
		return "", domain.ErrUnauthorized
	}
	token, session := newSession(user.ID)
	if err = s.Store.CreateSession(ctx, session, user.PasswordHash); err != nil {
		return "", err
	}
	return token, nil
}

func newSession(userID string) (string, domain.Session) {
	token := Token()
	return token, domain.Session{UserID: userID, TokenHash: TokenHash(token), ExpiresAt: time.Now().Add(SessionLifetime)}
}

func (s *Service) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if len(token) != 26 {
		return domain.User{}, domain.ErrUnauthorized
	}
	return s.Store.SessionUser(ctx, TokenHash(token))
}
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.Store.DeleteSession(ctx, TokenHash(token))
}
func (s *Service) CreateSite(ctx context.Context, userID, slug string) (domain.Site, error) {
	slug, err := domain.NormalizeSlug(slug)
	if err != nil {
		return domain.Site{}, err
	}
	site := domain.Site{ID: uuid.NewString(), UserID: userID, Slug: slug, CreatedAt: time.Now().UTC()}
	return site, s.Store.CreateSite(ctx, site)
}
func (s *Service) RenameSite(ctx context.Context, userID, id, slug string) (domain.Site, error) {
	slug, err := domain.NormalizeSlug(slug)
	if err != nil {
		return domain.Site{}, err
	}
	return s.Store.RenameSite(ctx, userID, id, slug)
}
func (s *Service) Invite(ctx context.Context, email string) (string, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return "", err
	}
	token := Token()
	return token, s.Store.Invite(ctx, email, TokenHash(token), time.Now().Add(7*24*time.Hour))
}
func (s *Service) IssueReset(ctx context.Context, email string) (string, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return "", err
	}
	token := Token()
	return token, s.Store.IssueReset(ctx, email, TokenHash(token), time.Now().Add(30*time.Minute))
}
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if len(token) != 26 {
		return domain.ErrUnauthorized
	}
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := s.Passwords.Hash(password)
	if err != nil {
		return err
	}
	return s.Store.ResetPassword(ctx, TokenHash(token), hash)
}

func (s *Service) Sites(ctx context.Context, userID string) ([]domain.Site, error) {
	return s.Store.Sites(ctx, userID)
}
func (s *Service) Site(ctx context.Context, userID, id string) (domain.Site, error) {
	return s.Store.Site(ctx, userID, id)
}
