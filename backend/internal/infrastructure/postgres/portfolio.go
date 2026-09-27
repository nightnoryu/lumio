package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/nightnoryu/go-kita/postgresql"

	"lumio/internal/domain"
)

func (s *Store) Draft(ctx context.Context, user, site string) (domain.Draft, error) {
	if _, err := s.Site(ctx, user, site); err != nil {
		return domain.Draft{}, err
	}
	var raw []byte
	err := s.DB.GetContext(ctx, &raw, `SELECT document FROM site_draft WHERE site_id=$1`, site)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EmptyDraft(), nil
	}
	if err != nil {
		return domain.Draft{}, err
	}
	var d domain.Draft
	err = json.Unmarshal(raw, &d)
	return d, err
}
func (s *Store) SaveDraft(ctx context.Context, user, site string, d domain.Draft) (domain.Draft, error) {
	err := s.transaction(ctx, func(tx postgresql.Transaction) error {
		var id string
		if err := tx.GetContext(ctx, &id, `SELECT id FROM site WHERE id=$1 AND user_id=$2 FOR UPDATE`, site, user); err != nil {
			return translate(err)
		}
		var version int64
		err := tx.GetContext(ctx, &version, `SELECT version FROM site_draft WHERE site_id=$1`, site)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if version != d.Version {
			return domain.ErrDraftConflict
		}
		for _, photo := range d.PhotoIDs() {
			var found string
			if err = tx.GetContext(ctx, &found, `SELECT id FROM photo WHERE id=$1 AND site_id=$2 AND status='ready' FOR SHARE`, photo, site); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return domain.InvalidDraft("Selected photographs must belong to this site and finish processing.")
				}
				return err
			}
		}
		d.Version++
		raw, err := json.Marshal(d)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO site_draft(site_id,version,document) VALUES($1,$2,$3) ON CONFLICT(site_id) DO UPDATE SET version=EXCLUDED.version,document=EXCLUDED.document,updated_at=now()`, site, d.Version, raw)
		return err
	})
	return d, err
}
