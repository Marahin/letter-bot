CREATE TABLE highscore_runs (
  id bigserial PRIMARY KEY,
  world varchar(100) NOT NULL,
  observed_at timestamptz NOT NULL,
  fetched_at timestamptz NOT NULL,
  pages int NOT NULL,
  rows int NOT NULL
);
CREATE INDEX highscore_runs_world_observed_idx ON highscore_runs (world, observed_at);

CREATE TABLE highscore_snapshots (
  id bigserial PRIMARY KEY,
  world varchar(100) NOT NULL,
  character_key text NOT NULL,
  character_name text NOT NULL,
  level int NOT NULL,
  experience bigint NOT NULL,
  vocation text NOT NULL DEFAULT '',
  -- A row covers [observed_at, last_seen_at]: runs that see the same value only move last_seen_at.
  observed_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX highscore_snapshots_lookup_uidx ON highscore_snapshots (world, character_key, observed_at);

CREATE TABLE reservation_experience (
  reservation_id bigint NOT NULL REFERENCES web_reservation(id) ON DELETE CASCADE,
  character_key text NOT NULL,
  character_name text NOT NULL,
  start_experience bigint,
  end_experience bigint,
  gain bigint,
  status text NOT NULL,
  computed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (reservation_id, character_key)
);
