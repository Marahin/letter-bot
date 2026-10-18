-- +goose Up
-- A member waiting for a respawn to be free (DECISIONS #105-#107). The queue
-- lives in Discord only. The length rules are in internal/core/queue.
CREATE TABLE spot_queue_entries (
  id bigserial PRIMARY KEY,
  guild_id text NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
  spot_id bigint NOT NULL REFERENCES web_spot(id) ON DELETE CASCADE,
  user_id text NOT NULL,
  length_minutes int NOT NULL CHECK (length_minutes > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  -- Set when the bot offered the free respawn; cleared when it was taken again.
  offered_at timestamptz
);

CREATE UNIQUE INDEX spot_queue_entries_guild_spot_user_uidx ON spot_queue_entries (guild_id, spot_id, user_id);

-- The one definition of "free" for the queue queries: no reservation of the
-- respawn overlaps [start_at, start_at + length], edges included, as in
-- SelectOverlappingReservations.
-- +goose StatementBegin
CREATE FUNCTION spot_queue_free(p_guild_id text, p_spot_id bigint, p_length_minutes int, p_start_at timestamptz)
RETURNS boolean
LANGUAGE sql STABLE
AS $$
  SELECT NOT EXISTS (
    SELECT 1
    FROM web_reservation r
    WHERE r.guild_id = p_guild_id
      AND r.spot_id = p_spot_id
      AND tstzrange(p_start_at, p_start_at + make_interval(mins => p_length_minutes), '[]')
        && tstzrange(r.start_at, r.end_at, '[]')
  )
$$;
-- +goose StatementEnd
