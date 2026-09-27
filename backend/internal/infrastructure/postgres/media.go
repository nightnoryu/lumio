package postgres

import (
	"context"

	"github.com/nightnoryu/go-kita/postgresql"

	"lumio/internal/app"
	"lumio/internal/domain"
)

const photoColumns = `id,site_id,name,size,content_type,status,original,watermark,width,height,variants,expires_at,attempts,lease,available_at`

func (s *Store) ReservePhoto(ctx context.Context, user string, p domain.Photo, l domain.MediaLimits) error {
	return s.transaction(ctx, func(tx postgresql.Transaction) error {
		var id string
		if err := tx.GetContext(ctx, &id, `SELECT id FROM site WHERE id=$1 AND user_id=$2 FOR UPDATE`, p.SiteID, user); err != nil {
			return translate(err)
		}
		var quota struct {
			Count int   `db:"count"`
			Bytes int64 `db:"bytes"`
		}
		if err := tx.GetContext(ctx, &quota, `SELECT count(*) AS count,coalesce(sum(size),0) AS bytes FROM photo WHERE site_id=$1`, p.SiteID); err != nil {
			return err
		}
		if quota.Count >= l.Photos || p.Size > l.StorageBytes-quota.Bytes {
			return domain.ErrMediaQuota
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO photo(id,site_id,name,size,content_type,status,original,watermark,expires_at) VALUES($1,$2,$3,$4,$5,'uploading',$6,$7,$8)`, p.ID, p.SiteID, p.Name, p.Size, p.ContentType, p.Original, p.Watermark, p.ExpiresAt)
		return err
	})
}
func (s *Store) Photos(ctx context.Context, user, site string) ([]domain.Photo, error) {
	if _, err := s.Site(ctx, user, site); err != nil {
		return nil, err
	}
	result := make([]domain.Photo, 0)
	err := s.DB.SelectContext(ctx, &result, `SELECT `+photoColumns+` FROM photo WHERE site_id=$1 AND status!='deleted' ORDER BY created_at,id`, site)
	return result, err
}
func (s *Store) Photo(ctx context.Context, user, site, id string) (domain.Photo, error) {
	var p domain.Photo
	err := s.DB.GetContext(ctx, &p, `SELECT `+photoColumns+` FROM photo WHERE id=$1 AND site_id=$2 AND status!='deleted' AND EXISTS(SELECT 1 FROM site WHERE id=$2 AND user_id=$3)`, id, site, user)
	return p, translate(err)
}
func (s *Store) QueuePhoto(ctx context.Context, user, site, id string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE photo SET status='queued',available_at=now() WHERE id=$1 AND site_id=$2 AND status='uploading' AND expires_at>now() AND EXISTS(SELECT 1 FROM site WHERE id=$2 AND user_id=$3)`, id, site, user)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return domain.ErrConflict
	}
	return err
}
func (s *Store) DeletePhoto(ctx context.Context, user, site, id string) error {
	return s.transaction(ctx, func(tx postgresql.Transaction) error {
		var locked string
		if err := tx.GetContext(ctx, &locked, `SELECT id FROM site WHERE id=$1 AND user_id=$2 FOR UPDATE`, site, user); err != nil {
			return translate(err)
		}
		var used bool
		if err := tx.GetContext(ctx, &used, `SELECT EXISTS(SELECT 1 FROM (SELECT document FROM site_draft WHERE site_id=$1 UNION ALL SELECT r.document FROM site_revision r JOIN site s ON s.published_revision=r.id WHERE s.id=$1) documents WHERE document->>'profilePhoto'=$2 OR document->>'coverPhoto'=$2 OR document->'photos' @> jsonb_build_array(jsonb_build_object('id',$2::text)))`, site, id); err != nil {
			return err
		}
		if used {
			return domain.InvalidDraft("This photograph is used by a portfolio. Remove it from the draft and save before deleting it; photos on the live site must be retained.")
		}
		result, err := tx.ExecContext(ctx, `UPDATE photo SET status='deleted',available_at=greatest(available_at,expires_at,now())+interval '1 minute' WHERE id=$1 AND site_id=$2`, id, site)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err == nil && n == 0 {
			return domain.ErrNotFound
		}
		return err
	})
}

func (s *Store) ClaimPhoto(ctx context.Context) (domain.Photo, error) {
	var p domain.Photo
	err := s.DB.GetContext(ctx, &p, `UPDATE photo SET status='processing',attempts=attempts+1,lease=$1,available_at=now()+interval '10 minutes' WHERE id=(SELECT id FROM photo WHERE status IN ('queued','processing') AND available_at<=now() ORDER BY available_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+photoColumns, app.Token())
	return p, translate(err)
}
func (s *Store) FinishPhoto(ctx context.Context, p domain.Photo, w, h int, variants string) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE photo SET status='ready',width=$3,height=$4,variants=$5 WHERE id=$1 AND lease=$2 AND status='processing'`, p.ID, p.Lease, w, h, variants)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return domain.ErrConflict
	}
	return err
}
func (s *Store) FailPhoto(ctx context.Context, p domain.Photo) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE photo SET status=CASE WHEN attempts>=3 THEN 'failed' ELSE 'queued' END,available_at=now()+interval '1 minute' * attempts WHERE id=$1 AND lease=$2 AND status='processing'`, p.ID, p.Lease)
	return err
}
func (s *Store) CleanupPhoto(ctx context.Context) (domain.Photo, error) {
	var p domain.Photo
	err := s.DB.GetContext(ctx, &p, `UPDATE photo SET status='deleted',lease=$1,available_at=now()+interval '10 minutes' WHERE id=(SELECT id FROM photo WHERE (status='deleted' AND available_at<=now()) OR (status='uploading' AND expires_at<now()-interval '1 minute') ORDER BY available_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+photoColumns, app.Token())
	return p, translate(err)
}
func (s *Store) PurgePhoto(ctx context.Context, p domain.Photo) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM photo WHERE id=$1 AND lease=$2 AND status='deleted'`, p.ID, p.Lease)
	return err
}
