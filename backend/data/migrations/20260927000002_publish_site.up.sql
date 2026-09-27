ALTER TABLE site_revision ADD COLUMN images jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(images)='object');
ALTER TABLE site_revision ADD CONSTRAINT site_revision_site_id_id_key UNIQUE(site_id,id);
ALTER TABLE site ADD COLUMN published_revision uuid;
ALTER TABLE site ADD CONSTRAINT site_published_revision_fk FOREIGN KEY(id,published_revision) REFERENCES site_revision(site_id,id);
