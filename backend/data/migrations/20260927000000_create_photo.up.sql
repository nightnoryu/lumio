CREATE TABLE photo (
 id uuid PRIMARY KEY,
 site_id uuid NOT NULL REFERENCES site(id),
 name text NOT NULL,
 size bigint NOT NULL CHECK(size>0),
 content_type text NOT NULL,
 status text NOT NULL CHECK(status IN ('uploading','queued','processing','ready','failed','deleted')),
 original text NOT NULL UNIQUE,
 watermark text NOT NULL DEFAULT '',
 width integer NOT NULL DEFAULT 0,
 height integer NOT NULL DEFAULT 0,
 variants text NOT NULL DEFAULT '[]',
 expires_at timestamptz NOT NULL,
 attempts integer NOT NULL DEFAULT 0,
 lease text NOT NULL DEFAULT '',
 available_at timestamptz NOT NULL DEFAULT now(),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX photo_site ON photo(site_id);
CREATE INDEX photo_jobs ON photo(available_at) WHERE status IN ('queued','processing','deleted','uploading');
