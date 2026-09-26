-- name: SpotTotals :many
-- Every totals query filters the same way: guild, start_at in [from_t, to_t), and optional
-- spot_id, user_id and character_key (zero value = all). The LIKE on lower(author)
-- lets the trigram index narrow a character filter before the exact unnest check.
WITH r AS (
  SELECT r.id, r.spot_id, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM web_reservation r
  WHERE r.guild_id = @guild_id::text
    AND r.start_at >= @from_t::timestamptz
    AND r.start_at < @to_t::timestamptz
    AND (@spot_id::bigint = 0 OR r.spot_id = @spot_id::bigint)
    AND (@user_id::text = '' OR r.author_discord_id = @user_id::text)
    AND (@character_key::text = '' OR (lower(r.author) LIKE '%' || @character_key::text || '%'
      AND EXISTS (SELECT 1 FROM unnest(string_to_array(r.author, '/')) AS n(name) WHERE lower(btrim(n.name)) = @character_key::text)))
), e AS (
  SELECT re.reservation_id, sum(re.gain)::bigint AS gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
    AND (@character_key::text = '' OR re.character_key = @character_key::text)
  GROUP BY re.reservation_id
)
SELECT s.id AS spot_id,
       s.name::text AS name,
       (s.archived_at IS NOT NULL)::boolean AS archived,
       count(*)::bigint AS reservations,
       coalesce(sum(r.secs), 0)::bigint AS seconds,
       count(e.reservation_id)::bigint AS exp_reservations,
       coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
       coalesce(sum(e.gain), 0)::bigint AS exp
FROM r
JOIN web_spot s ON s.id = r.spot_id
LEFT JOIN e ON e.reservation_id = r.id
GROUP BY s.id, s.name, s.archived_at;

-- name: PlayerTotals :many
-- The name is looked up per player after grouping: an ordered array_agg would force a sort of every row.
WITH r AS (
  SELECT r.id, r.author_discord_id, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM web_reservation r
  WHERE r.guild_id = @guild_id::text
    AND r.start_at >= @from_t::timestamptz
    AND r.start_at < @to_t::timestamptz
    AND r.author_discord_id <> ''
    AND (@spot_id::bigint = 0 OR r.spot_id = @spot_id::bigint)
    AND (@user_id::text = '' OR r.author_discord_id = @user_id::text)
    AND (@character_key::text = '' OR (lower(r.author) LIKE '%' || @character_key::text || '%'
      AND EXISTS (SELECT 1 FROM unnest(string_to_array(r.author, '/')) AS n(name) WHERE lower(btrim(n.name)) = @character_key::text)))
), e AS (
  SELECT re.reservation_id, sum(re.gain)::bigint AS gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
    AND (@character_key::text = '' OR re.character_key = @character_key::text)
  GROUP BY re.reservation_id
), g AS (
  SELECT r.author_discord_id,
         count(*)::bigint AS reservations,
         coalesce(sum(r.secs), 0)::bigint AS seconds,
         count(e.reservation_id)::bigint AS exp_reservations,
         coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp
  FROM r
  LEFT JOIN e ON e.reservation_id = r.id
  GROUP BY r.author_discord_id
)
SELECT g.author_discord_id::text AS user_id,
       coalesce((SELECT w.author FROM web_reservation w
                 WHERE w.guild_id = @guild_id::text AND w.author_discord_id = g.author_discord_id
                 ORDER BY w.end_at DESC LIMIT 1), '')::text AS name,
       g.reservations::bigint AS reservations,
       g.seconds::bigint AS seconds,
       g.exp_reservations::bigint AS exp_reservations,
       g.exp_seconds::bigint AS exp_seconds,
       g.exp::bigint AS exp
FROM g;

-- name: CharacterTotals :many
-- The inner GROUP BY dedupes a character named twice in one author text; it sorts 1-3 names per
-- reservation instead of every row.
WITH r AS (
  SELECT r.id, r.author, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM web_reservation r
  WHERE r.guild_id = @guild_id::text
    AND r.start_at >= @from_t::timestamptz
    AND r.start_at < @to_t::timestamptz
    AND (@spot_id::bigint = 0 OR r.spot_id = @spot_id::bigint)
    AND (@user_id::text = '' OR r.author_discord_id = @user_id::text)
    AND (@character_key::text = '' OR lower(r.author) LIKE '%' || @character_key::text || '%')
), c AS (
  SELECT r.id, r.secs, n.character_key, n.character_name
  FROM r
  CROSS JOIN LATERAL (
    SELECT lower(btrim(m.name)) AS character_key, min(btrim(m.name)) AS character_name
    FROM unnest(string_to_array(r.author, '/')) AS m(name)
    WHERE btrim(m.name) <> ''
    GROUP BY lower(btrim(m.name))
  ) AS n
  WHERE @character_key::text = '' OR n.character_key = @character_key::text
)
SELECT c.character_key::text AS character_key,
       max(c.character_name)::text AS name,
       count(*)::bigint AS reservations,
       coalesce(sum(c.secs), 0)::bigint AS seconds,
       count(re.gain)::bigint AS exp_reservations,
       coalesce(sum(c.secs) FILTER (WHERE re.gain IS NOT NULL), 0)::bigint AS exp_seconds,
       coalesce(sum(re.gain), 0)::bigint AS exp
FROM c
LEFT JOIN reservation_experience re
  ON re.reservation_id = c.id AND re.character_key = c.character_key AND re.status = 'ok'
GROUP BY c.character_key;

-- name: Daily :many
WITH r AS (
  SELECT r.id, (r.start_at AT TIME ZONE @tz::text)::date AS day, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM web_reservation r
  WHERE r.guild_id = @guild_id::text
    AND r.start_at >= @from_t::timestamptz
    AND r.start_at < @to_t::timestamptz
    AND (@spot_id::bigint = 0 OR r.spot_id = @spot_id::bigint)
    AND (@user_id::text = '' OR r.author_discord_id = @user_id::text)
    AND (@character_key::text = '' OR (lower(r.author) LIKE '%' || @character_key::text || '%'
      AND EXISTS (SELECT 1 FROM unnest(string_to_array(r.author, '/')) AS n(name) WHERE lower(btrim(n.name)) = @character_key::text)))
), e AS (
  SELECT re.reservation_id, sum(re.gain)::bigint AS gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
    AND (@character_key::text = '' OR re.character_key = @character_key::text)
  GROUP BY re.reservation_id
)
SELECT r.day::date AS day,
       count(*)::bigint AS reservations,
       coalesce(sum(r.secs), 0)::bigint AS seconds,
       count(e.reservation_id)::bigint AS exp_reservations,
       coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
       coalesce(sum(e.gain), 0)::bigint AS exp
FROM r
LEFT JOIN e ON e.reservation_id = r.id
GROUP BY r.day
ORDER BY r.day;

-- name: ReservationDays :many
-- A loose index scan: one index probe per day instead of reading every reservation.
WITH RECURSIVE d AS (
  (SELECT w.start_at FROM web_reservation w
   WHERE w.guild_id = @guild_id::text AND w.start_at >= @from_t::timestamptz AND w.start_at < @to_t::timestamptz
   ORDER BY w.start_at LIMIT 1)
  UNION ALL
  SELECT (SELECT w.start_at FROM web_reservation w
          WHERE w.guild_id = @guild_id::text
            AND w.start_at >= ((date_trunc('day', d.start_at AT TIME ZONE @tz::text) + interval '1 day') AT TIME ZONE @tz::text)
            AND w.start_at < @to_t::timestamptz
          ORDER BY w.start_at LIMIT 1)
  FROM d
  WHERE d.start_at IS NOT NULL
)
SELECT (d.start_at AT TIME ZONE @tz::text)::date AS day
FROM d
WHERE d.start_at IS NOT NULL
ORDER BY day;

-- name: LatestPlayerName :one
SELECT author::text AS name
FROM web_reservation
WHERE guild_id = @guild_id::text
  AND author_discord_id = @user_id::text
ORDER BY end_at DESC
LIMIT 1;

-- name: CharacterReservations :many
SELECT r.id, r.spot_id, s.name::text AS spot_name, r.author::text AS author, r.start_at, r.end_at, re.status, re.gain
FROM web_reservation r
JOIN web_spot s ON s.id = r.spot_id
LEFT JOIN reservation_experience re ON re.reservation_id = r.id AND re.character_key = @character_key::text
WHERE r.guild_id = @guild_id::text
  AND r.start_at >= @from_t::timestamptz
  AND r.start_at < @to_t::timestamptz
  AND lower(r.author) LIKE '%' || @character_key::text || '%'
  AND EXISTS (SELECT 1 FROM unnest(string_to_array(r.author, '/')) AS n(name) WHERE lower(btrim(n.name)) = @character_key::text)
ORDER BY r.start_at DESC, r.id DESC
LIMIT @lim::int;
