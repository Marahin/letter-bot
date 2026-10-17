-- The cross-guild stats read only public guilds: premium, with the bot present and a world set.
-- Every query shares the CTEs eg (those guilds, narrowed to world_key), r (their reservations
-- in [from_t, to_t), narrowed to spawn_key), e (experience per reservation) and l (the loot
-- session per reservation). A world and a spawn are keyed by lower(btrim(name)).

-- name: PublicWorlds :many
SELECT lower(btrim(gw.world_name))::text AS world_key,
       mode() WITHIN GROUP (ORDER BY btrim(gw.world_name))::text AS name
FROM guilds g
JOIN guilds_world gw ON gw.guild_id = g.guild_id
WHERE g.bot_present AND (g.premium OR g.premium_forever) AND btrim(gw.world_name) <> ''
GROUP BY lower(btrim(gw.world_name))
ORDER BY name;

-- name: WorldTotals :many
WITH eg AS (
  SELECT g.guild_id, btrim(gw.world_name) AS world_name, lower(btrim(gw.world_name)) AS world_key
  FROM guilds g
  JOIN guilds_world gw ON gw.guild_id = g.guild_id
  WHERE g.bot_present AND (g.premium OR g.premium_forever) AND btrim(gw.world_name) <> ''
    AND (@world_key::text = '' OR lower(btrim(gw.world_name)) = @world_key::text)
), r AS (
  SELECT r.id, eg.world_key, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM eg
  JOIN web_reservation r ON r.guild_id = eg.guild_id
  JOIN web_spot s ON s.id = r.spot_id
  WHERE r.start_at >= @from_t::timestamptz AND r.start_at < @to_t::timestamptz
    AND (@spawn_key::text = '' OR lower(btrim(s.name)) = @spawn_key::text)
), e AS (
  SELECT re.reservation_id, sum(re.gain)::bigint AS gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
  GROUP BY re.reservation_id
), l AS (
  SELECT rl.reservation_id, ls.duration_seconds::bigint AS secs, ls.balance AS profit, ls.loot, ls.supplies
  FROM reservation_loot rl
  JOIN r ON r.id = rl.reservation_id
  JOIN loot_sessions ls ON ls.id = rl.session_id
), t AS (
  SELECT r.world_key,
         count(*)::bigint AS reservations,
         coalesce(sum(r.secs), 0)::bigint AS seconds,
         count(e.reservation_id)::bigint AS exp_reservations,
         coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp,
         count(l.reservation_id)::bigint AS loot_reservations,
         coalesce(sum(l.secs), 0)::bigint AS loot_seconds,
         coalesce(sum(l.profit), 0)::bigint AS profit,
         coalesce(sum(l.loot), 0)::bigint AS loot,
         coalesce(sum(l.supplies), 0)::bigint AS supplies
  FROM r
  LEFT JOIN e ON e.reservation_id = r.id
  LEFT JOIN l ON l.reservation_id = r.id
  GROUP BY r.world_key
), w AS (
  SELECT eg.world_key, mode() WITHIN GROUP (ORDER BY eg.world_name) AS name, count(*) AS guilds
  FROM eg
  GROUP BY eg.world_key
)
SELECT w.world_key::text AS world_key, w.name::text AS name, w.guilds::bigint AS guilds,
       coalesce(t.reservations, 0)::bigint AS reservations,
       coalesce(t.seconds, 0)::bigint AS seconds,
       coalesce(t.exp_reservations, 0)::bigint AS exp_reservations,
       coalesce(t.exp_seconds, 0)::bigint AS exp_seconds,
       coalesce(t.exp, 0)::bigint AS exp,
       coalesce(t.loot_reservations, 0)::bigint AS loot_reservations,
       coalesce(t.loot_seconds, 0)::bigint AS loot_seconds,
       coalesce(t.profit, 0)::bigint AS profit,
       coalesce(t.loot, 0)::bigint AS loot,
       coalesce(t.supplies, 0)::bigint AS supplies
FROM w
LEFT JOIN t ON t.world_key = w.world_key
ORDER BY coalesce(t.reservations, 0) DESC, lower(w.name);

-- name: WorldGuildTotals :many
-- Every guild of the world(s), with or without reservations in the range.
WITH eg AS (
  SELECT g.guild_id, g.name AS guild_name, g.icon
  FROM guilds g
  JOIN guilds_world gw ON gw.guild_id = g.guild_id
  WHERE g.bot_present AND (g.premium OR g.premium_forever) AND btrim(gw.world_name) <> ''
    AND (@world_key::text = '' OR lower(btrim(gw.world_name)) = @world_key::text)
), r AS (
  SELECT r.id, r.guild_id, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM eg
  JOIN web_reservation r ON r.guild_id = eg.guild_id
  JOIN web_spot s ON s.id = r.spot_id
  WHERE r.start_at >= @from_t::timestamptz AND r.start_at < @to_t::timestamptz
    AND (@spawn_key::text = '' OR lower(btrim(s.name)) = @spawn_key::text)
), e AS (
  SELECT re.reservation_id, sum(re.gain)::bigint AS gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
  GROUP BY re.reservation_id
), l AS (
  SELECT rl.reservation_id, ls.duration_seconds::bigint AS secs, ls.balance AS profit, ls.loot, ls.supplies
  FROM reservation_loot rl
  JOIN r ON r.id = rl.reservation_id
  JOIN loot_sessions ls ON ls.id = rl.session_id
), t AS (
  SELECT r.guild_id,
         count(*)::bigint AS reservations,
         coalesce(sum(r.secs), 0)::bigint AS seconds,
         count(e.reservation_id)::bigint AS exp_reservations,
         coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp,
         count(l.reservation_id)::bigint AS loot_reservations,
         coalesce(sum(l.secs), 0)::bigint AS loot_seconds,
         coalesce(sum(l.profit), 0)::bigint AS profit,
         coalesce(sum(l.loot), 0)::bigint AS loot,
         coalesce(sum(l.supplies), 0)::bigint AS supplies
  FROM r
  LEFT JOIN e ON e.reservation_id = r.id
  LEFT JOIN l ON l.reservation_id = r.id
  GROUP BY r.guild_id
)
SELECT eg.guild_id::text AS guild_id, eg.guild_name::text AS name, eg.icon::text AS icon,
       coalesce(t.reservations, 0)::bigint AS reservations,
       coalesce(t.seconds, 0)::bigint AS seconds,
       coalesce(t.exp_reservations, 0)::bigint AS exp_reservations,
       coalesce(t.exp_seconds, 0)::bigint AS exp_seconds,
       coalesce(t.exp, 0)::bigint AS exp,
       coalesce(t.loot_reservations, 0)::bigint AS loot_reservations,
       coalesce(t.loot_seconds, 0)::bigint AS loot_seconds,
       coalesce(t.profit, 0)::bigint AS profit,
       coalesce(t.loot, 0)::bigint AS loot,
       coalesce(t.supplies, 0)::bigint AS supplies
FROM eg
LEFT JOIN t ON t.guild_id = eg.guild_id
ORDER BY coalesce(t.seconds, 0) DESC, lower(eg.guild_name), eg.guild_id
LIMIT 500;

-- name: SpawnTotals :many
-- Orders like SpotTotals; worlds counts the worlds that booked the spawn in the range.
WITH eg AS (
  SELECT g.guild_id, lower(btrim(gw.world_name)) AS world_key
  FROM guilds g
  JOIN guilds_world gw ON gw.guild_id = g.guild_id
  WHERE g.bot_present AND (g.premium OR g.premium_forever) AND btrim(gw.world_name) <> ''
    AND (@world_key::text = '' OR lower(btrim(gw.world_name)) = @world_key::text)
), r AS (
  SELECT r.id, eg.world_key, lower(btrim(s.name)) AS spawn_key, btrim(s.name) AS spawn_name,
         extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM eg
  JOIN web_reservation r ON r.guild_id = eg.guild_id
  JOIN web_spot s ON s.id = r.spot_id
  WHERE r.start_at >= @from_t::timestamptz AND r.start_at < @to_t::timestamptz
    AND (@spawn_key::text = '' OR lower(btrim(s.name)) = @spawn_key::text)
), e AS (
  SELECT re.reservation_id, sum(re.gain)::bigint AS gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
  GROUP BY re.reservation_id
), l AS (
  SELECT rl.reservation_id, ls.duration_seconds::bigint AS secs, ls.balance AS profit, ls.loot, ls.supplies
  FROM reservation_loot rl
  JOIN r ON r.id = rl.reservation_id
  JOIN loot_sessions ls ON ls.id = rl.session_id
), g AS (
  SELECT r.spawn_key,
         mode() WITHIN GROUP (ORDER BY r.spawn_name) AS name,
         count(DISTINCT r.world_key)::bigint AS worlds,
         count(*)::bigint AS reservations,
         coalesce(sum(r.secs), 0)::bigint AS seconds,
         count(e.reservation_id)::bigint AS exp_reservations,
         coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp,
         count(l.reservation_id)::bigint AS loot_reservations,
         coalesce(sum(l.secs), 0)::bigint AS loot_seconds,
         coalesce(sum(l.profit), 0)::bigint AS profit,
         coalesce(sum(l.loot), 0)::bigint AS loot,
         coalesce(sum(l.supplies), 0)::bigint AS supplies
  FROM r
  LEFT JOIN e ON e.reservation_id = r.id
  LEFT JOIN l ON l.reservation_id = r.id
  GROUP BY r.spawn_key
), m AS (
  SELECT g.*, CASE @sort_key::text
         WHEN 'worlds' THEN g.worlds::float8
         WHEN 'reservations' THEN g.reservations::float8
         WHEN 'exp' THEN CASE WHEN g.exp_reservations > 0 THEN g.exp::float8 END
         WHEN 'exp_h' THEN CASE WHEN g.exp_seconds > 0 THEN g.exp::float8 * 3600 / g.exp_seconds END
         WHEN 'profit' THEN CASE WHEN g.loot_reservations > 0 THEN g.profit::float8 END
         WHEN 'profit_h' THEN CASE WHEN g.loot_seconds > 0 THEN g.profit::float8 * 3600 / g.loot_seconds END
         WHEN 'name' THEN NULL
         ELSE g.seconds::float8
       END AS figure
  FROM g
  WHERE (@min_exp_seconds::bigint = 0 OR g.exp_seconds >= @min_exp_seconds::bigint)
    AND (@min_loot_seconds::bigint = 0 OR g.loot_seconds >= @min_loot_seconds::bigint)
)
SELECT m.spawn_key::text AS spawn_key, m.name::text AS name, m.worlds::bigint AS worlds,
       m.reservations::bigint AS reservations,
       m.seconds::bigint AS seconds,
       m.exp_reservations::bigint AS exp_reservations,
       m.exp_seconds::bigint AS exp_seconds,
       m.exp::bigint AS exp,
       m.loot_reservations::bigint AS loot_reservations,
       m.loot_seconds::bigint AS loot_seconds,
       m.profit::bigint AS profit,
       m.loot::bigint AS loot,
       m.supplies::bigint AS supplies,
       count(*) OVER ()::bigint AS total_rows
FROM m
ORDER BY CASE WHEN @sort_key::text = 'name' AND @ascending::boolean THEN lower(m.name) END ASC,
         CASE WHEN @sort_key::text = 'name' AND NOT @ascending::boolean THEN lower(m.name) END DESC,
         CASE WHEN @ascending::boolean THEN m.figure END ASC NULLS LAST,
         CASE WHEN NOT @ascending::boolean THEN m.figure END DESC NULLS LAST,
         lower(m.name), m.spawn_key
LIMIT NULLIF(@lim::int, 0);

-- name: SpawnSpotTotals :many
WITH eg AS (
  SELECT g.guild_id, g.name AS guild_name, btrim(gw.world_name) AS world_name, lower(btrim(gw.world_name)) AS world_key
  FROM guilds g
  JOIN guilds_world gw ON gw.guild_id = g.guild_id
  WHERE g.bot_present AND (g.premium OR g.premium_forever) AND btrim(gw.world_name) <> ''
    AND (@world_key::text = '' OR lower(btrim(gw.world_name)) = @world_key::text)
), r AS (
  SELECT r.id, r.spot_id, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM eg
  JOIN web_reservation r ON r.guild_id = eg.guild_id
  JOIN web_spot s ON s.id = r.spot_id
  WHERE r.start_at >= @from_t::timestamptz AND r.start_at < @to_t::timestamptz
    AND (@spawn_key::text = '' OR lower(btrim(s.name)) = @spawn_key::text)
), e AS (
  SELECT re.reservation_id, sum(re.gain)::bigint AS gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
  GROUP BY re.reservation_id
), l AS (
  SELECT rl.reservation_id, ls.duration_seconds::bigint AS secs, ls.balance AS profit, ls.loot, ls.supplies
  FROM reservation_loot rl
  JOIN r ON r.id = rl.reservation_id
  JOIN loot_sessions ls ON ls.id = rl.session_id
)
SELECT eg.world_key::text AS world_key, eg.world_name::text AS world_name,
       eg.guild_id::text AS guild_id, eg.guild_name::text AS guild_name,
       s.id AS spot_id, s.name::text AS spot_name, (s.archived_at IS NOT NULL)::boolean AS archived,
       count(*)::bigint AS reservations,
       coalesce(sum(r.secs), 0)::bigint AS seconds,
       count(e.reservation_id)::bigint AS exp_reservations,
       coalesce(sum(r.secs) FILTER (WHERE e.reservation_id IS NOT NULL), 0)::bigint AS exp_seconds,
       coalesce(sum(e.gain), 0)::bigint AS exp,
       count(l.reservation_id)::bigint AS loot_reservations,
       coalesce(sum(l.secs), 0)::bigint AS loot_seconds,
       coalesce(sum(l.profit), 0)::bigint AS profit,
       coalesce(sum(l.loot), 0)::bigint AS loot,
       coalesce(sum(l.supplies), 0)::bigint AS supplies
FROM r
JOIN web_spot s ON s.id = r.spot_id
JOIN eg ON eg.guild_id = s.guild_id
LEFT JOIN e ON e.reservation_id = r.id
LEFT JOIN l ON l.reservation_id = r.id
GROUP BY eg.world_key, eg.world_name, eg.guild_id, eg.guild_name, s.id, s.name, s.archived_at
ORDER BY seconds DESC, lower(eg.world_name), lower(eg.guild_name), s.id
LIMIT 500;

-- name: SpawnName :one
-- Archived respawns count: their reservations are still in the stats.
SELECT mode() WITHIN GROUP (ORDER BY btrim(s.name))::text AS name
FROM guilds g
JOIN guilds_world gw ON gw.guild_id = g.guild_id
JOIN web_spot s ON s.guild_id = g.guild_id
WHERE g.bot_present AND (g.premium OR g.premium_forever) AND btrim(gw.world_name) <> ''
  AND lower(btrim(s.name)) = @spawn_key::text
HAVING count(*) > 0;

-- name: WorldCharacterBoards :many
-- The world's characters with the most experience and the most profit (equal shares). The name
-- and the guild come from the character's latest reservation.
WITH eg AS (
  SELECT g.guild_id
  FROM guilds g
  JOIN guilds_world gw ON gw.guild_id = g.guild_id
  WHERE g.bot_present AND (g.premium OR g.premium_forever) AND btrim(gw.world_name) <> ''
    AND (@world_key::text = '' OR lower(btrim(gw.world_name)) = @world_key::text)
), r AS (
  SELECT r.id, r.author, r.start_at, extract(epoch FROM r.end_at - r.start_at)::bigint AS secs
  FROM eg
  JOIN web_reservation r ON r.guild_id = eg.guild_id
  JOIN web_spot s ON s.id = r.spot_id
  WHERE r.start_at >= @from_t::timestamptz AND r.start_at < @to_t::timestamptz
    AND (@spawn_key::text = '' OR lower(btrim(s.name)) = @spawn_key::text)
), c AS (
  SELECT r.id, r.start_at, r.secs, n.character_key
  FROM r
  CROSS JOIN LATERAL (
    SELECT DISTINCT lower(btrim(m.name)) AS character_key
    FROM unnest(string_to_array(r.author, '/')) AS m(name)
    WHERE btrim(m.name) <> ''
  ) AS n
), e AS (
  SELECT re.reservation_id, re.character_key, re.gain
  FROM reservation_experience re
  JOIN r ON r.id = re.reservation_id
  WHERE re.status = 'ok'
), l AS (
  SELECT rl.reservation_id, lm.character_key, ls.duration_seconds::bigint AS secs,
         floor(ls.balance::numeric / ls.members)::bigint AS profit,
         floor(ls.loot::numeric / ls.members)::bigint AS loot,
         floor(ls.supplies::numeric / ls.members)::bigint AS supplies
  FROM reservation_loot rl
  JOIN r ON r.id = rl.reservation_id
  JOIN loot_sessions ls ON ls.id = rl.session_id
  JOIN loot_session_members lm ON lm.session_id = ls.id
), g AS (
  SELECT c.character_key,
         (max(ARRAY[extract(epoch FROM c.start_at)::bigint, c.id]))[2] AS last_id,
         count(*)::bigint AS reservations,
         coalesce(sum(c.secs), 0)::bigint AS seconds,
         count(e.gain)::bigint AS exp_reservations,
         coalesce(sum(c.secs) FILTER (WHERE e.gain IS NOT NULL), 0)::bigint AS exp_seconds,
         coalesce(sum(e.gain), 0)::bigint AS exp,
         count(l.reservation_id)::bigint AS loot_reservations,
         coalesce(sum(l.secs), 0)::bigint AS loot_seconds,
         coalesce(sum(l.profit), 0)::bigint AS profit,
         coalesce(sum(l.loot), 0)::bigint AS loot,
         coalesce(sum(l.supplies), 0)::bigint AS supplies
  FROM c
  LEFT JOIN e ON e.reservation_id = c.id AND e.character_key = c.character_key
  LEFT JOIN l ON l.reservation_id = c.id AND l.character_key = c.character_key
  GROUP BY c.character_key
), b AS (
  (SELECT 'exp'::text AS board, g.exp::float8 AS figure, g.*
   FROM g WHERE g.exp_reservations > 0
   ORDER BY g.exp DESC, g.character_key LIMIT @lim::int)
  UNION ALL
  (SELECT 'profit'::text AS board, g.profit::float8 AS figure, g.*
   FROM g WHERE g.loot_reservations > 0
   ORDER BY g.profit DESC, g.character_key LIMIT @lim::int)
)
SELECT b.board::text AS board,
       b.character_key::text AS character_key,
       coalesce((SELECT min(btrim(n.name)) FROM unnest(string_to_array(w.author, '/')) AS n(name)
        WHERE lower(btrim(n.name)) = b.character_key), '')::text AS name,
       w.guild_id::text AS guild_id,
       b.reservations::bigint AS reservations,
       b.seconds::bigint AS seconds,
       b.exp_reservations::bigint AS exp_reservations,
       b.exp_seconds::bigint AS exp_seconds,
       b.exp::bigint AS exp,
       b.loot_reservations::bigint AS loot_reservations,
       b.loot_seconds::bigint AS loot_seconds,
       b.profit::bigint AS profit,
       b.loot::bigint AS loot,
       b.supplies::bigint AS supplies
FROM b
JOIN web_reservation w ON w.id = b.last_id
ORDER BY b.board, b.figure DESC, b.character_key;
