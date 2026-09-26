-- name: ListTrackedWorlds :many
SELECT DISTINCT gw.world_name::text AS world
FROM guilds_world gw
JOIN guilds g ON g.guild_id = gw.guild_id
WHERE g.premium OR g.premium_forever
ORDER BY world;

-- name: ListTrackedCharacterKeys :many
SELECT DISTINCT lower(btrim(n.name))::text AS character_key
FROM web_reservation r
JOIN guilds_world gw ON gw.guild_id = r.guild_id
JOIN guilds g ON g.guild_id = r.guild_id
CROSS JOIN LATERAL unnest(string_to_array(r.author, '/')) AS n(name)
WHERE gw.world_name = @world::text
  AND (g.premium OR g.premium_forever)
  AND r.start_at >= @start_from::timestamptz
  AND r.end_at >= @since::timestamptz
  AND btrim(n.name) <> ''
ORDER BY character_key;

-- name: LatestSnapshots :many
SELECT DISTINCT ON (character_key) id, world, character_key, character_name, level, experience, vocation, observed_at, last_seen_at
FROM highscore_snapshots
WHERE world = @world::text
  AND character_key = ANY(@keys::text[])
ORDER BY character_key, observed_at DESC, id DESC;

-- name: InsertSnapshots :exec
INSERT INTO highscore_snapshots (world, character_key, character_name, level, experience, vocation, observed_at, last_seen_at)
SELECT @world::text,
       unnest(@character_keys::text[]),
       unnest(@character_names::text[]),
       unnest(@levels::int[]),
       unnest(@experiences::bigint[]),
       unnest(@vocations::text[]),
       @observed_at::timestamptz,
       @observed_at::timestamptz;

-- name: TouchSnapshots :exec
UPDATE highscore_snapshots
SET last_seen_at = GREATEST(last_seen_at, @seen_at::timestamptz)
WHERE id = ANY(@ids::bigint[]);

-- name: InsertRun :exec
INSERT INTO highscore_runs (world, observed_at, fetched_at, pages, rows)
VALUES (@world::text, @observed_at, @fetched_at, @pages, @rows);

-- name: FirstRunObservedAt :one
SELECT min(observed_at)::timestamptz AS observed_at
FROM highscore_runs
WHERE world = @world::text;

-- name: FirstRunObservedAtOrAfter :one
SELECT observed_at
FROM highscore_runs
WHERE world = @world::text
  AND observed_at >= @t::timestamptz
ORDER BY observed_at
LIMIT 1;

-- name: SnapshotAtOrBefore :one
SELECT id, world, character_key, character_name, level, experience, vocation, observed_at, last_seen_at
FROM highscore_snapshots
WHERE world = @world::text
  AND character_key = @character_key::text
  AND observed_at <= @t::timestamptz
ORDER BY observed_at DESC, id DESC
LIMIT 1;

-- name: FirstSnapshotAfter :one
SELECT id, world, character_key, character_name, level, experience, vocation, observed_at, last_seen_at
FROM highscore_snapshots
WHERE world = @world::text
  AND character_key = @character_key::text
  AND observed_at > @from_t::timestamptz
  AND observed_at <= @to_t::timestamptz
ORDER BY observed_at, id
LIMIT 1;

-- name: PendingReservations :many
SELECT r.id, r.author, r.start_at, r.end_at
FROM web_reservation r
JOIN guilds_world gw ON gw.guild_id = r.guild_id
JOIN guilds g ON g.guild_id = r.guild_id
WHERE gw.world_name = @world::text
  AND (g.premium OR g.premium_forever)
  AND r.start_at >= @start_from::timestamptz
  AND r.end_at >= @end_from::timestamptz
  AND r.end_at <= @end_to::timestamptz
  AND NOT EXISTS (SELECT 1 FROM reservation_experience re WHERE re.reservation_id = r.id)
ORDER BY r.end_at, r.id;

-- name: InsertReservationExperience :exec
INSERT INTO reservation_experience (reservation_id, character_key, character_name, start_experience, end_experience, gain, status)
SELECT t.reservation_id,
       t.character_key,
       t.character_name,
       CASE WHEN t.has_start THEN t.start_experience END,
       CASE WHEN t.has_end THEN t.end_experience END,
       CASE WHEN t.status = 'ok' THEN t.gain END,
       t.status
FROM (SELECT unnest(@reservation_ids::bigint[]) AS reservation_id,
             unnest(@character_keys::text[]) AS character_key,
             unnest(@character_names::text[]) AS character_name,
             unnest(@start_experiences::bigint[]) AS start_experience,
             unnest(@has_starts::boolean[]) AS has_start,
             unnest(@end_experiences::bigint[]) AS end_experience,
             unnest(@has_ends::boolean[]) AS has_end,
             unnest(@gains::bigint[]) AS gain,
             unnest(@statuses::text[]) AS status) AS t
ON CONFLICT (reservation_id, character_key) DO NOTHING;

-- name: SnapshotHistory :many
SELECT id, world, character_key, character_name, level, experience, vocation, observed_at, last_seen_at
FROM highscore_snapshots
WHERE world = @world::text
  AND character_key = @character_key::text
  AND observed_at >= @from_t::timestamptz
  AND observed_at <= @to_t::timestamptz
ORDER BY observed_at, id;
