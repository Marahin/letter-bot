-- A snapshot row now covers [observed_at, last_seen_at]: runs that saw the same value only move last_seen_at.
ALTER TABLE highscore_snapshots ADD COLUMN last_seen_at timestamptz;
UPDATE highscore_snapshots SET last_seen_at = observed_at WHERE last_seen_at IS NULL;
ALTER TABLE highscore_snapshots ALTER COLUMN last_seen_at SET NOT NULL;
