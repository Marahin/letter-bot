CREATE TABLE web_users (
  discord_user_id text PRIMARY KEY,
  username text NOT NULL,
  global_name text NOT NULL DEFAULT '',
  avatar text NOT NULL DEFAULT '',
  access_token text NOT NULL DEFAULT '',
  refresh_token text NOT NULL DEFAULT '',
  token_expiry timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Layout required by scs pgxstore.
CREATE TABLE web_sessions (
  token text PRIMARY KEY,
  data bytea NOT NULL,
  expiry timestamptz NOT NULL
);
CREATE INDEX web_sessions_expiry_idx ON web_sessions (expiry);
