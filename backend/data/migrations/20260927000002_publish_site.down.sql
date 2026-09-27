ALTER TABLE site DROP CONSTRAINT site_published_revision_fk;
ALTER TABLE site DROP COLUMN published_revision;
ALTER TABLE site_revision DROP CONSTRAINT site_revision_site_id_id_key;
ALTER TABLE site_revision DROP COLUMN images;
