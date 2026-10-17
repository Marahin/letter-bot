-- name: InsertSession :one
INSERT INTO loot_sessions (content_hash, started_at, ended_at, duration_seconds, members, loot, supplies, balance, damage, healing)
VALUES (@content_hash, @started_at, @ended_at, @duration_seconds, @members, @loot, @supplies, @balance, @damage, @healing)
ON CONFLICT (content_hash) DO NOTHING
RETURNING id;

-- name: SessionIDByHash :one
SELECT id FROM loot_sessions WHERE content_hash = @content_hash;

-- name: InsertMembers :exec
INSERT INTO loot_session_members (session_id, character_key, character_name, position, leader, loot, supplies, balance, damage, healing)
SELECT @session_id::bigint, unnest(@keys::text[]), unnest(@names::text[]), unnest(@positions::int[]), unnest(@leaders::bool[]),
       unnest(@loots::bigint[]), unnest(@supplies::bigint[]), unnest(@balances::bigint[]), unnest(@damages::bigint[]), unnest(@healings::bigint[])
ON CONFLICT (session_id, character_key) DO NOTHING;

-- name: MatchCandidates :many
-- start_from (the session start minus the longest reservation) bounds the scan on start_at.
SELECT r.id, r.guild_id::text AS guild_id, g.name AS guild_name, r.spot_id, s.name::text AS spot_name,
       r.author::text AS author, r.start_at, r.end_at
FROM guilds g
JOIN web_reservation r ON r.guild_id = g.guild_id
JOIN web_spot s ON s.id = r.spot_id
WHERE g.bot_present AND (g.premium OR g.premium_forever)
  AND r.start_at > @start_from::timestamptz
  AND r.start_at < @to_t::timestamptz
  AND r.end_at > @from_t::timestamptz
  AND EXISTS (SELECT 1 FROM unnest(string_to_array(r.author, '/')) AS n(name)
              WHERE lower(btrim(n.name)) = ANY(@keys::text[]))
ORDER BY r.start_at DESC, r.id DESC
LIMIT 50;

-- name: DeleteOtherPicks :exec
DELETE FROM reservation_loot WHERE session_id = @session_id AND reservation_id <> @reservation_id;

-- name: PickSession :one
-- On an equal overlap the reservation keeps its session: a later paste cannot displace it.
INSERT INTO reservation_loot (reservation_id, session_id, overlap_seconds)
VALUES (@reservation_id, @session_id, @overlap_seconds)
ON CONFLICT (reservation_id) DO UPDATE
SET session_id = EXCLUDED.session_id, overlap_seconds = EXCLUDED.overlap_seconds, picked_at = now()
WHERE reservation_loot.overlap_seconds < EXCLUDED.overlap_seconds
   OR reservation_loot.session_id = EXCLUDED.session_id
RETURNING session_id;
