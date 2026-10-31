-- name: DeleteGuildRoles :exec
DELETE FROM guild_roles
WHERE guild_id = @guild_id;

-- name: InsertGuildRoles :exec
INSERT INTO guild_roles (guild_id, role_id, name, color, position)
SELECT @guild_id::text,
       unnest(@role_ids::text[]),
       unnest(@names::text[]),
       unnest(@colors::int[]),
       unnest(@positions::int[]);

-- Highest role first, as Discord lists them.
-- name: SelectGuildRoles :many
SELECT role_id, name, color, position
FROM guild_roles
WHERE guild_id = @guild_id
ORDER BY position DESC, lower(name), role_id;
