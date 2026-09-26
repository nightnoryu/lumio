CREATE TABLE session (
 token_hash text PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
