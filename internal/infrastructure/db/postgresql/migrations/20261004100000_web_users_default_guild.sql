-- +goose Up
ALTER TABLE web_users ADD COLUMN default_guild_id text NOT NULL DEFAULT '';
