CREATE TABLE "user" (
 id uuid PRIMARY KEY,
 email text NOT NULL UNIQUE CHECK (email = lower(email)),
 password_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
