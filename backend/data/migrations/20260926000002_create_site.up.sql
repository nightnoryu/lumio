CREATE TABLE site (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
 slug varchar(63) NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
 created_at timestamptz NOT NULL DEFAULT now()
);
