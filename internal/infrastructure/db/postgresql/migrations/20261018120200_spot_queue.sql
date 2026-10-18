-- +goose Up
-- A member waiting for a respawn to be free (DECISIONS #105-#107). The queue
-- lives in Discord only.
CREATE TABLE spot_queue_entries (
  id bigserial PRIMARY KEY,
  guild_id text NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
  spot_id bigint NOT NULL REFERENCES web_spot(id) ON DELETE CASCADE,
  user_id text NOT NULL,
  length_minutes int NOT NULL CHECK (length_minutes BETWEEN 30 AND 180 AND length_minutes % 30 = 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  -- Set when the bot offered the free respawn; cleared when it was taken again.
  offered_at timestamptz
);

CREATE UNIQUE INDEX spot_queue_entries_guild_spot_user_uidx ON spot_queue_entries (guild_id, spot_id, user_id);
