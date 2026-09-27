package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/nightnoryu/go-kita/postgresql"

	"lumio/internal/domain"
)

func (s *Store) Publish(ctx context.Context, user, site string, version int64) (int64, error) {
	err := s.transaction(ctx, func(tx postgresql.Transaction) error {
		var id string
		if err := tx.GetContext(ctx, &id, `SELECT id FROM site WHERE id=$1 AND user_id=$2 FOR UPDATE`, site, user); err != nil {
			return translate(err)
		}
		var raw []byte
		if err := tx.GetContext(ctx, &raw, `SELECT document FROM site_draft WHERE site_id=$1`, site); err != nil {
			return translate(err)
		}
		var d domain.Draft
		if err := json.Unmarshal(raw, &d); err != nil {
			return err
		}
		if d.Version != version {
			return domain.ErrDraftConflict
		}
		if err := d.ValidatePublication(); err != nil {
			return err
		}
		var current int64
		if err := tx.GetContext(ctx, &current, `SELECT coalesce((r.document->>'version')::bigint,0) FROM site s LEFT JOIN site_revision r ON r.id=s.published_revision WHERE s.id=$1`, site); err != nil {
			return err
		}
		if current == version {
			return nil
		}
		images, err := publicationImages(ctx, tx, site, d.PhotoIDs())
		if err != nil {
			return err
		}
		manifest, err := json.Marshal(images)
		if err != nil {
			return err
		}
		revision := uuid.NewString()
		if _, err = tx.ExecContext(ctx, `INSERT INTO site_revision(id,site_id,document,images) VALUES($1,$2,$3,$4)`, revision, site, raw, manifest); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE site SET published_revision=$2 WHERE id=$1`, site, revision)
		return err
	})
	return version, err
}
func publicationImages(ctx context.Context, tx postgresql.Transaction, site string, ids []string) (map[string]domain.PublishedImage, error) {
	images := make(map[string]domain.PublishedImage, len(ids))
	for _, id := range ids {
		var p domain.Photo
		if err := tx.GetContext(ctx, &p, `SELECT `+photoColumns+` FROM photo WHERE id=$1 AND site_id=$2 AND status='ready' FOR SHARE`, id, site); err != nil {
			if errors.Is(translate(err), domain.ErrNotFound) {
				return nil, domain.InvalidDraft("Selected photographs must belong to this site and finish processing before publication.")
			}
			return nil, err
		}
		image, err := domain.SnapshotImage(p)
		if err != nil {
			return nil, err
		}
		images[id] = image
	}
	return images, nil
}
func (s *Store) Unpublish(ctx context.Context, user, site string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE site SET published_revision=NULL WHERE id=$1 AND user_id=$2`, site, user)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrNotFound
	}
	return err
}
func (s *Store) PublicationVersion(ctx context.Context, user, site string) (int64, error) {
	var version int64
	err := s.DB.GetContext(ctx, &version, `SELECT coalesce((r.document->>'version')::bigint,0) FROM site s LEFT JOIN site_revision r ON r.id=s.published_revision WHERE s.id=$1 AND s.user_id=$2`, site, user)
	return version, translate(err)
}
func (s *Store) Published(ctx context.Context, slug string) (domain.Revision, error) {
	var row struct {
		ID       string `db:"id"`
		Document []byte `db:"document"`
		Images   []byte `db:"images"`
	}
	err := s.DB.GetContext(ctx, &row, `SELECT r.id,r.document,r.images FROM site s JOIN site_revision r ON r.id=s.published_revision AND r.site_id=s.id WHERE s.slug=$1`, slug)
	if err != nil {
		return domain.Revision{}, translate(err)
	}
	revision := domain.Revision{ID: row.ID}
	if err = json.Unmarshal(row.Document, &revision.Draft); err != nil {
		return revision, err
	}
	err = json.Unmarshal(row.Images, &revision.Images)
	return revision, err
}
