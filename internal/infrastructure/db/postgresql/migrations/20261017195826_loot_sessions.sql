-- +goose Up
-- A Party Hunt session counted from the Loot Calculator. The pasted text is not kept.
CREATE TABLE loot_sessions (
  id bigserial PRIMARY KEY,
  content_hash bytea NOT NULL,
  -- The signed-in first submitter's Discord user id, '' when anonymous.
  submitted_by text NOT NULL DEFAULT '',
  started_at timestamptz NOT NULL,
  ended_at timestamptz NOT NULL,
  duration_seconds integer NOT NULL,
  members integer NOT NULL,
  loot bigint NOT NULL,
  supplies bigint NOT NULL,
  balance bigint NOT NULL,
  damage bigint NOT NULL,
  healing bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT loot_sessions_content_hash_key UNIQUE (content_hash),
  CONSTRAINT loot_sessions_interval_check CHECK (ended_at > started_at AND duration_seconds > 0),
  CONSTRAINT loot_sessions_members_check CHECK (members > 0)
);

CREATE TABLE loot_session_members (
  session_id bigint NOT NULL REFERENCES loot_sessions(id) ON DELETE CASCADE,
  character_key text NOT NULL,
  character_name text NOT NULL,
  position integer NOT NULL,
  leader boolean NOT NULL DEFAULT false,
  loot bigint NOT NULL,
  supplies bigint NOT NULL,
  balance bigint NOT NULL,
  damage bigint NOT NULL,
  healing bigint NOT NULL,
  PRIMARY KEY (session_id, character_key)
);

-- A reservation keeps one session, and a session counts for one reservation.
CREATE TABLE reservation_loot (
  reservation_id bigint PRIMARY KEY REFERENCES web_reservation(id) ON DELETE CASCADE,
  session_id bigint NOT NULL REFERENCES loot_sessions(id) ON DELETE CASCADE,
  overlap_seconds integer NOT NULL CHECK (overlap_seconds > 0),
  -- The reservation's author submitted the session: no one else's session replaces it.
  by_author boolean NOT NULL DEFAULT false,
  picked_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT reservation_loot_session_key UNIQUE (session_id)
);
