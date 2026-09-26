CREATE TABLE invitation (
 token_hash text PRIMARY KEY,
 email text NOT NULL,
 expires_at timestamptz NOT NULL
);
