package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nightnoryu/go-kita/postgresql"

	"lumio/internal/domain"
)

type Store struct {
	DB postgresql.TransactionalClient
}

type userRow struct {
	ID           string `db:"id"`
	Email        string `db:"email"`
	PasswordHash string `db:"password_hash"`
}

func (r userRow) domain() domain.User {
	return domain.User{ID: r.ID, Email: r.Email, PasswordHash: r.PasswordHash}
}

type siteRow struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Slug      string    `db:"slug"`
	CreatedAt time.Time `db:"created_at"`
}

func (r siteRow) domain() domain.Site {
	return domain.Site{ID: r.ID, UserID: r.UserID, Slug: r.Slug, CreatedAt: r.CreatedAt}
}

func (s *Store) transaction(ctx context.Context, fn func(postgresql.Transaction) error) error {
	tx, err := s.DB.BeginTransaction(ctx, nil)
	if err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return errors.Join(err, rollback(tx))
	}
	return translate(tx.Commit())
}
func rollback(tx postgresql.Transaction) error {
	err := tx.Rollback()
	if errors.Is(err, sql.ErrTxDone) {
		return nil
	}
	return err
}
func translate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func (s *Store) Register(ctx context.Context, user domain.User, invitation string, session domain.Session) error {
	return s.transaction(ctx, func(tx postgresql.Transaction) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM invitation WHERE token_hash=$1 AND email=$2 AND expires_at>now()`, invitation, user.Email)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return domain.ErrInvitation
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO "user" (id,email,password_hash) VALUES ($1,$2,$3)`, user.ID, user.Email, user.PasswordHash)
		if err != nil {
			return translate(err)
		}
		return insertSession(ctx, tx, session)
	})
}
func insertSession(ctx context.Context, client postgresql.ClientContext, session domain.Session) error {
	_, err := client.ExecContext(ctx, `INSERT INTO session (token_hash,user_id,expires_at) VALUES ($1,$2,$3)`, session.TokenHash, session.UserID, session.ExpiresAt)
	return err
}
func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	var row userRow
	err := s.DB.GetContext(ctx, &row, `SELECT id,email,password_hash FROM "user" WHERE email=$1`, email)
	return row.domain(), translate(err)
}
func (s *Store) CreateSession(ctx context.Context, session domain.Session, passwordHash string) error {
	return s.transaction(ctx, func(tx postgresql.Transaction) error {
		var current string
		// Serialize with password resets so an in-flight login cannot restore a revoked session.
		err := tx.GetContext(ctx, &current, `SELECT password_hash FROM "user" WHERE id=$1 FOR UPDATE`, session.UserID)
		if err != nil {
			return translate(err)
		}
		if current != passwordHash {
			return domain.ErrUnauthorized
		}
		return insertSession(ctx, tx, session)
	})
}
func (s *Store) SessionUser(ctx context.Context, hash string) (domain.User, error) {
	var row userRow
	err := s.DB.GetContext(ctx, &row, `SELECT u.id,u.email,u.password_hash FROM "user" u JOIN session s ON s.user_id=u.id WHERE s.token_hash=$1 AND s.expires_at>now()`, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrUnauthorized
	}
	return row.domain(), err
}
func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM session WHERE token_hash=$1`, hash)
	return err
}
func (s *Store) CreateSite(ctx context.Context, site domain.Site) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO site (id,user_id,slug,created_at) VALUES ($1,$2,$3,$4)`, site.ID, site.UserID, site.Slug, site.CreatedAt)
	return translate(err)
}
func (s *Store) Sites(ctx context.Context, userID string) ([]domain.Site, error) {
	var rows []siteRow
	err := s.DB.SelectContext(ctx, &rows, `SELECT id,user_id,slug,created_at FROM site WHERE user_id=$1 ORDER BY created_at,id`, userID)
	sites := make([]domain.Site, 0, len(rows))
	for _, row := range rows {
		sites = append(sites, row.domain())
	}
	return sites, err
}
func (s *Store) Site(ctx context.Context, userID, id string) (domain.Site, error) {
	var row siteRow
	err := s.DB.GetContext(ctx, &row, `SELECT id,user_id,slug,created_at FROM site WHERE id=$1 AND user_id=$2`, id, userID)
	return row.domain(), translate(err)
}
func (s *Store) RenameSite(ctx context.Context, userID, id, slug string) (domain.Site, error) {
	var row siteRow
	err := s.DB.GetContext(ctx, &row, `UPDATE site SET slug=$3 WHERE id=$1 AND user_id=$2 RETURNING id,user_id,slug,created_at`, id, userID, slug)
	return row.domain(), translate(err)
}
func (s *Store) Invite(ctx context.Context, email, hash string, expires time.Time) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO invitation (token_hash,email,expires_at) VALUES ($1,$2,$3)`, hash, email, expires)
	return err
}
func (s *Store) IssueReset(ctx context.Context, email, hash string, expires time.Time) error {
	return s.transaction(ctx, func(tx postgresql.Transaction) error {
		var id string
		err := tx.GetContext(ctx, &id, `SELECT id FROM "user" WHERE email=$1 FOR UPDATE`, email)
		if err != nil {
			return translate(err)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO password_reset (token_hash,user_id,expires_at) VALUES ($1,$2,$3) ON CONFLICT (user_id) DO UPDATE SET token_hash=EXCLUDED.token_hash,expires_at=EXCLUDED.expires_at`, hash, id, expires)
		return err
	})
}
func (s *Store) ResetPassword(ctx context.Context, tokenHash, passwordHash string) error {
	return s.transaction(ctx, func(tx postgresql.Transaction) error {
		var id string
		err := tx.GetContext(ctx, &id, `SELECT user_id FROM password_reset WHERE token_hash=$1 AND expires_at>now()`, tokenHash)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrUnauthorized
		}
		if err != nil {
			return err
		}
		var lockedID string
		err = tx.GetContext(ctx, &lockedID, `SELECT id FROM "user" WHERE id=$1 FOR UPDATE`, id)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM password_reset WHERE token_hash=$1 AND expires_at>now()`, tokenHash)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return domain.ErrUnauthorized
		}
		_, err = tx.ExecContext(ctx, `UPDATE "user" SET password_hash=$2 WHERE id=$1`, id, passwordHash)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM session WHERE user_id=$1`, id)
		return err
	})
}
