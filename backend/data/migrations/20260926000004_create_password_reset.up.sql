CREATE TABLE password_reset (
 token_hash text PRIMARY KEY,
 user_id uuid NOT NULL UNIQUE REFERENCES "user"(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL
);
