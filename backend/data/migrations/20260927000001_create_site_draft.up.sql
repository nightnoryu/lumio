CREATE TABLE site_draft (
 site_id uuid PRIMARY KEY REFERENCES site(id) ON DELETE CASCADE,
 version bigint NOT NULL CHECK(version > 0),
 document jsonb NOT NULL CHECK(jsonb_typeof(document)='object'),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE site_revision (
 id uuid PRIMARY KEY,
 site_id uuid NOT NULL REFERENCES site(id) ON DELETE CASCADE,
 document jsonb NOT NULL CHECK(jsonb_typeof(document)='object'),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION reject_revision_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Site revisions are immutable'; END;
$$;
CREATE TRIGGER immutable_site_revision BEFORE UPDATE ON site_revision
 FOR EACH ROW EXECUTE FUNCTION reject_revision_update();
