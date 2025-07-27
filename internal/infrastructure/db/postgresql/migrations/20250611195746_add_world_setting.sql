
-- Create "guilds_world" table
CREATE TABLE "public"."guilds_world" (
  "id" bigserial NOT NULL,
  "guild_id" character varying(255) NOT NULL,
  "world_name" character varying(100) NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "guilds_world_guild_id_key" UNIQUE ("guild_id")
);


