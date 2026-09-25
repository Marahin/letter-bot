CREATE TABLE guilds (
  guild_id text PRIMARY KEY,
  name text NOT NULL DEFAULT '',
  icon text NOT NULL DEFAULT '',
  owner_id text NOT NULL DEFAULT '',
  bot_present boolean NOT NULL DEFAULT false,
  premium boolean NOT NULL DEFAULT false,
  premium_forever boolean NOT NULL DEFAULT false,
  command_channel_id text NOT NULL DEFAULT '',
  summary_channel_id text NOT NULL DEFAULT '',
  manage_role_ids text[] NOT NULL DEFAULT '{}',
  view_role_ids text[] NOT NULL DEFAULT '{}',
  reserve_role_ids text[] NOT NULL DEFAULT '{}',
  overbook_role_ids text[] NOT NULL DEFAULT '{}',
  resync_requested_at timestamptz,
  synced_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE guild_channels (
  guild_id text NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
  channel_id text NOT NULL,
  name text NOT NULL DEFAULT '',
  type int NOT NULL DEFAULT 0,
  parent_id text NOT NULL DEFAULT '',
  position int NOT NULL DEFAULT 0,
  PRIMARY KEY (guild_id, channel_id)
);

CREATE TABLE guild_roles (
  guild_id text NOT NULL REFERENCES guilds(guild_id) ON DELETE CASCADE,
  role_id text NOT NULL,
  name text NOT NULL DEFAULT '',
  color int NOT NULL DEFAULT 0,
  position int NOT NULL DEFAULT 0,
  PRIMARY KEY (guild_id, role_id)
);

-- Celesta Community is premium forever (DECISIONS #5).
INSERT INTO guilds (guild_id, name, premium, premium_forever)
VALUES ('806152499760201738', 'Celesta Community', true, true)
ON CONFLICT (guild_id) DO UPDATE SET premium = true, premium_forever = true;
