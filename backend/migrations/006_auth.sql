CREATE TABLE owner_account (
 id integer PRIMARY KEY CHECK (id=1),
 username text NOT NULL,
 password_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE owner_sessions (
 token_hash text PRIMARY KEY,
 owner_id integer NOT NULL REFERENCES owner_account(id),
 expires_at timestamptz NOT NULL
);
CREATE INDEX owner_sessions_expiry ON owner_sessions(expires_at);
