-- name: UpsertWebUser :one
INSERT INTO web_users (discord_user_id, username, global_name, avatar)
VALUES (@discord_user_id, @username, @global_name, @avatar)
ON CONFLICT (discord_user_id) DO UPDATE
SET username = EXCLUDED.username,
    global_name = EXCLUDED.global_name,
    avatar = EXCLUDED.avatar,
    updated_at = now()
RETURNING discord_user_id, username, global_name, avatar, default_guild_id, created_at, updated_at;

-- name: SelectWebUser :one
SELECT discord_user_id, username, global_name, avatar, default_guild_id, created_at, updated_at
FROM web_users
WHERE discord_user_id = @discord_user_id;

-- name: SaveWebUserToken :execrows
UPDATE web_users
SET access_token = @access_token,
    refresh_token = @refresh_token,
    token_expiry = @token_expiry,
    updated_at = now()
WHERE discord_user_id = @discord_user_id;

-- name: SelectWebUserToken :one
SELECT access_token, refresh_token, token_expiry
FROM web_users
WHERE discord_user_id = @discord_user_id;

-- name: SetWebUserDefaultGuild :execrows
UPDATE web_users
SET default_guild_id = @default_guild_id,
    updated_at = now()
WHERE discord_user_id = @discord_user_id;
