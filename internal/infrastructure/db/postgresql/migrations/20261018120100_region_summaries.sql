-- +goose Up
-- One summary channel per region (DECISIONS #95-#99). On by default.
ALTER TABLE guilds ADD COLUMN region_summaries boolean NOT NULL DEFAULT true;

-- The Discord category and the index message the bot made for a guild.
CREATE TABLE summary_layouts (
  guild_id text PRIMARY KEY REFERENCES guilds(guild_id) ON DELETE CASCADE,
  category_id text NOT NULL DEFAULT '',
  index_channel_id text NOT NULL DEFAULT '',
  index_message_id text NOT NULL DEFAULT '',
  index_hash text NOT NULL DEFAULT '',
  -- Set when Discord refused a channel (permissions, channel limit): the guild
  -- keeps the single summary until then.
  blocked_until timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- A region channel the bot made or adopted, with the messages it posted there.
CREATE TABLE summary_region_channels (
  guild_id text NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
  region text NOT NULL,
  channel_id text NOT NULL,
  message_ids text[] NOT NULL DEFAULT '{}',
  content_hash text NOT NULL DEFAULT '',
  retired_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (guild_id, region)
);
