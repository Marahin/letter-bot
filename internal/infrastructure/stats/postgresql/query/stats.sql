-- name: SpotTotals :many
-- Every totals query filters the same way: guild, start_at in [from_t, to_t), and optional
-- spot_id, user_id and character_key (zero value = all). The LIKE on lower(author)
-- lets the trigram index narrow a character filter before the exact unnest check.
-- They share the ordering too: sort_key's figure (NULL = no data, last in both directions), then
-- the name. lim 0 = every row; total_rows counts the rows before the limit.
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
), g AS (
  SELECT s.id AS spot_id, s.name::text AS name, (s.archived_at IS NOT NULL)::boolean AS archived,
         count(*)::bigint AS reservations,
         coalesce(sum(r.secs), 0)::bigint AS seconds,
         count(e.reservation_id)::bigint AS exp_reservations,
         coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp
  FROM r
  JOIN web_spot s ON s.id = r.spot_id
  LEFT JOIN e ON e.reservation_id = r.id
  GROUP BY s.id, s.name, s.archived_at
), m AS (
  SELECT g.*, CASE @sort_key::text
         WHEN 'reservations' THEN g.reservations::float8
         WHEN 'exp' THEN CASE WHEN g.exp_reservations > 0 THEN g.exp::float8 END
         WHEN 'exp_h' THEN CASE WHEN g.exp_seconds > 0 THEN g.exp::float8 * 3600 / g.exp_seconds END
         WHEN 'name' THEN NULL
         ELSE g.seconds::float8
       END AS figure
  FROM g
)
SELECT m.spot_id::bigint AS spot_id, m.name::text AS name, m.archived::boolean AS archived,
       m.reservations::bigint AS reservations,
       m.seconds::bigint AS seconds,
       m.exp_reservations::bigint AS exp_reservations,
       m.exp_seconds::bigint AS exp_seconds,
       m.exp::bigint AS exp,
       count(*) OVER ()::bigint AS total_rows
FROM m
ORDER BY CASE WHEN @sort_key::text = 'name' AND @ascending::boolean THEN lower(m.name) END ASC,
         CASE WHEN @sort_key::text = 'name' AND NOT @ascending::boolean THEN lower(m.name) END DESC,
         CASE WHEN @ascending::boolean THEN m.figure END ASC NULLS LAST,
         CASE WHEN NOT @ascending::boolean THEN m.figure END DESC NULLS LAST,
         lower(m.name), m.spot_id
LIMIT NULLIF(@lim::int, 0);


-- name: PlayerTotals :many
-- The name is looked up after the limit: an ordered array_agg would force a sort of every row.
-- A name sort needs it before, so that lookup runs per player only then; other sorts break ties by user id.
WITH r AS (
  SELECT r.id, r.author_discord_id, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM web_reservation r
  WHERE r.guild_id = @guild_id::text
    AND r.start_at >= @from_t::timestamptz
    AND r.start_at < @to_t::timestamptz
    AND (@spot_id::bigint = 0 OR r.spot_id = @spot_id::bigint)
    AND (@user_id::text = '' OR r.author_discord_id = @user_id::text)
    AND r.author_discord_id <> ''
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
  SELECT r.author_discord_id AS user_id,
         CASE WHEN @sort_key::text = 'name' THEN (SELECT w.author FROM web_reservation w
           WHERE w.guild_id = @guild_id::text AND w.author_discord_id = r.author_discord_id
           ORDER BY w.end_at DESC LIMIT 1) END AS name,
         count(*)::bigint AS reservations,
         coalesce(sum(r.secs), 0)::bigint AS seconds,
         count(e.reservation_id)::bigint AS exp_reservations,
         coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp
  FROM r
  LEFT JOIN e ON e.reservation_id = r.id
  GROUP BY r.author_discord_id
), m AS (
  SELECT g.*, CASE @sort_key::text
         WHEN 'reservations' THEN g.reservations::float8
         WHEN 'exp' THEN CASE WHEN g.exp_reservations > 0 THEN g.exp::float8 END
         WHEN 'exp_h' THEN CASE WHEN g.exp_seconds > 0 THEN g.exp::float8 * 3600 / g.exp_seconds END
         WHEN 'name' THEN NULL
         ELSE g.seconds::float8
       END AS figure
  FROM g
)
SELECT m.user_id::text AS user_id,
       coalesce(m.name, (SELECT w.author FROM web_reservation w
                 WHERE w.guild_id = @guild_id::text AND w.author_discord_id = m.user_id
                 ORDER BY w.end_at DESC LIMIT 1), '')::text AS name,
       m.reservations::bigint AS reservations,
       m.seconds::bigint AS seconds,
       m.exp_reservations::bigint AS exp_reservations,
       m.exp_seconds::bigint AS exp_seconds,
       m.exp::bigint AS exp,
       count(*) OVER ()::bigint AS total_rows
FROM m
ORDER BY CASE WHEN @sort_key::text = 'name' AND @ascending::boolean THEN lower(m.name) END ASC,
         CASE WHEN @sort_key::text = 'name' AND NOT @ascending::boolean THEN lower(m.name) END DESC,
         CASE WHEN @ascending::boolean THEN m.figure END ASC NULLS LAST,
         CASE WHEN NOT @ascending::boolean THEN m.figure END DESC NULLS LAST,
         lower(m.name), m.user_id
LIMIT NULLIF(@lim::int, 0);

-- name: CharacterTotals :many
-- The DISTINCT dedupes a character named twice in one author text; it sorts 1-3 names per
-- reservation instead of every row. The name is the spelling in the latest reservation (by start),
-- looked up by id after the limit, or before it for a name sort.
WITH r AS (
  SELECT r.id, r.author, r.start_at, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM web_reservation r
  WHERE r.guild_id = @guild_id::text
    AND r.start_at >= @from_t::timestamptz
    AND r.start_at < @to_t::timestamptz
    AND (@spot_id::bigint = 0 OR r.spot_id = @spot_id::bigint)
    AND (@user_id::text = '' OR r.author_discord_id = @user_id::text)
    AND (@character_key::text = '' OR lower(r.author) LIKE '%' || @character_key::text || '%')
), c AS (
  SELECT r.id, r.start_at, r.secs, n.character_key
  FROM r
  CROSS JOIN LATERAL (
    SELECT DISTINCT lower(btrim(m.name)) AS character_key
    FROM unnest(string_to_array(r.author, '/')) AS m(name)
    WHERE btrim(m.name) <> ''
  ) AS n
  WHERE @character_key::text = '' OR n.character_key = @character_key::text
), e AS (
  SELECT re.reservation_id, re.character_key, re.gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
), g AS (
  SELECT c.character_key,
         (max(ARRAY[extract(epoch FROM c.start_at)::bigint, c.id]))[2] AS last_id,
         count(*)::bigint AS reservations,
         coalesce(sum(c.secs), 0)::bigint AS seconds,
         count(e.gain)::bigint AS exp_reservations,
         coalesce(sum(c.secs) FILTER (WHERE e.gain IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp
  FROM c
  LEFT JOIN e ON e.reservation_id = c.id AND e.character_key = c.character_key
  GROUP BY c.character_key
), m AS (
  SELECT g.*,
         CASE WHEN @sort_key::text = 'name' THEN (SELECT min(btrim(n.name)) FROM web_reservation w CROSS JOIN LATERAL unnest(string_to_array(w.author, '/')) AS n(name)
        WHERE w.id = g.last_id AND lower(btrim(n.name)) = g.character_key) END AS name,
         CASE @sort_key::text
         WHEN 'reservations' THEN g.reservations::float8
         WHEN 'exp' THEN CASE WHEN g.exp_reservations > 0 THEN g.exp::float8 END
         WHEN 'exp_h' THEN CASE WHEN g.exp_seconds > 0 THEN g.exp::float8 * 3600 / g.exp_seconds END
         WHEN 'name' THEN NULL
         ELSE g.seconds::float8
       END AS figure
  FROM g
)
SELECT m.character_key::text AS character_key,
       coalesce(m.name, (SELECT min(btrim(n.name)) FROM web_reservation w CROSS JOIN LATERAL unnest(string_to_array(w.author, '/')) AS n(name)
        WHERE w.id = m.last_id AND lower(btrim(n.name)) = m.character_key), '')::text AS name,
       m.reservations::bigint AS reservations,
       m.seconds::bigint AS seconds,
       m.exp_reservations::bigint AS exp_reservations,
       m.exp_seconds::bigint AS exp_seconds,
       m.exp::bigint AS exp,
       count(*) OVER ()::bigint AS total_rows
FROM m
ORDER BY CASE WHEN @sort_key::text = 'name' AND @ascending::boolean THEN lower(m.name) END ASC,
         CASE WHEN @sort_key::text = 'name' AND NOT @ascending::boolean THEN lower(m.name) END DESC,
         CASE WHEN @ascending::boolean THEN m.figure END ASC NULLS LAST,
         CASE WHEN NOT @ascending::boolean THEN m.figure END DESC NULLS LAST,
         m.character_key
LIMIT NULLIF(@lim::int, 0);

-- name: CharacterLeaderboards :many
-- Both overview character leaderboards and the number of characters from one pass over the
-- reservations. The count row always comes back; board is NULL when neither leaderboard has a row.
WITH r AS (
  SELECT r.id, r.author, r.start_at, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM web_reservation r
  WHERE r.guild_id = @guild_id::text
    AND r.start_at >= @from_t::timestamptz
    AND r.start_at < @to_t::timestamptz
    AND (@spot_id::bigint = 0 OR r.spot_id = @spot_id::bigint)
    AND (@user_id::text = '' OR r.author_discord_id = @user_id::text)
    AND (@character_key::text = '' OR lower(r.author) LIKE '%' || @character_key::text || '%')
), c AS (
  SELECT r.id, r.start_at, r.secs, n.character_key
  FROM r
  CROSS JOIN LATERAL (
    SELECT DISTINCT lower(btrim(m.name)) AS character_key
    FROM unnest(string_to_array(r.author, '/')) AS m(name)
    WHERE btrim(m.name) <> ''
  ) AS n
  WHERE @character_key::text = '' OR n.character_key = @character_key::text
), e AS (
  SELECT re.reservation_id, re.character_key, re.gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
), g AS (
  SELECT c.character_key,
         (max(ARRAY[extract(epoch FROM c.start_at)::bigint, c.id]))[2] AS last_id,
         count(*)::bigint AS reservations,
         coalesce(sum(c.secs), 0)::bigint AS seconds,
         count(e.gain)::bigint AS exp_reservations,
         coalesce(sum(c.secs) FILTER (WHERE e.gain IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp
  FROM c
  LEFT JOIN e ON e.reservation_id = c.id AND e.character_key = c.character_key
  GROUP BY c.character_key
), b AS (
  (SELECT 'exp'::text AS board, g.exp::float8 AS figure, g.*
   FROM g WHERE g.exp_reservations > 0
   ORDER BY g.exp DESC, g.character_key LIMIT @lim::int)
  UNION ALL
  (SELECT 'exp_h'::text AS board, g.exp::float8 * 3600 / g.exp_seconds AS figure, g.*
   FROM g WHERE g.exp_seconds > 0 AND g.exp_seconds >= @min_exp_seconds::bigint
   ORDER BY g.exp::float8 / g.exp_seconds DESC, g.character_key LIMIT @lim::int)
)
SELECT t.total_rows::bigint AS total_rows,
       b.board,
       b.character_key,
       coalesce((SELECT min(btrim(n.name)) FROM web_reservation w CROSS JOIN LATERAL unnest(string_to_array(w.author, '/')) AS n(name)
        WHERE w.id = b.last_id AND lower(btrim(n.name)) = b.character_key), '')::text AS name,
       b.reservations,
       b.seconds,
       b.exp_reservations,
       b.exp_seconds,
       b.exp
FROM (SELECT count(*) AS total_rows FROM g) AS t
LEFT JOIN b ON true
ORDER BY b.board, b.figure DESC, b.character_key;

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
