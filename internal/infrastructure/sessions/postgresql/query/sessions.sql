-- name: GetUserBySession :one
SELECT u.discord_id, u.username, u.avatar_url, u.created_at
FROM users u
         JOIN sessions s ON u.discord_id = s.discord_id
WHERE s.session_cookie = $1;

-- name: InsertSession :exec
INSERT INTO sessions (session_cookie, discord_id, expires_at)
VALUES ($1, $2, $3);

-- name: DeleteOldSessions :exec
DELETE FROM sessions
WHERE expires_at < NOW();

-- name: GetUserGuildsBySession :many
SELECT g.guild_id, g.guild_name, g.created_at
FROM guilds g
         JOIN user_roles ur ON g.guild_id = ur.guild_id
         JOIN sessions s ON ur.discord_id = s.discord_id
WHERE s.session_cookie = $1
GROUP BY g.guild_id, g.guild_name, g.created_at;

-- name: GetUserRolesBySession :many
SELECT r.role_id, r.role_name, ur.guild_id, ur.created_at
FROM roles r
         JOIN user_roles ur ON r.role_id = ur.role_id
         JOIN sessions s ON ur.discord_id = s.discord_id
WHERE s.session_cookie = $1
ORDER BY ur.guild_id, r.role_id;

-- name: UpsertRole :exec
INSERT INTO roles (role_id, role_name, guild_id, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (role_id) DO UPDATE
    SET role_name = EXCLUDED.role_name,
        guild_id = EXCLUDED.guild_id,
        created_at = EXCLUDED.created_at;

-- name: UpsertUserRole :exec
INSERT INTO user_roles (discord_id, guild_id, role_id, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (discord_id, guild_id, role_id) DO UPDATE
    SET created_at = EXCLUDED.created_at;
