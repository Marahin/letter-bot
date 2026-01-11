-- Create table to track summary channel message IDs
CREATE TABLE "public"."summary_messages" (
  "id" bigserial NOT NULL,
  "guild_id" character varying(255) NOT NULL,
  "channel_id" character varying(255) NOT NULL,
  "message_id" character varying(255) NOT NULL,
  "message_type" character varying(50) NOT NULL,
  "message_order" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);

-- Index for fast lookups by guild
CREATE INDEX "summary_messages_guild_id_idx" ON "public"."summary_messages" ("guild_id");

-- Unique constraint to prevent duplicate messages
CREATE UNIQUE INDEX "summary_messages_guild_channel_type_order_uniq" 
  ON "public"."summary_messages" ("guild_id", "channel_id", "message_type", "message_order");
