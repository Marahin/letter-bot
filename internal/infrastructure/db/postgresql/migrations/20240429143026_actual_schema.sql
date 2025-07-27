-- Add btree_gist extension
CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;

-- Create "web_spot" table
CREATE TABLE "public"."web_spot" ("id" bigserial NOT NULL, "name" character varying(120) NOT NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));

-- Create "web_reservation" table
CREATE TABLE "public"."web_reservation" ("id" bigserial NOT NULL, "author" character varying(200) NOT NULL, "created_at" timestamptz NOT NULL, "start_at" timestamptz NOT NULL, "end_at" timestamptz NOT NULL, "spot_id" bigint NOT NULL, "guild_id" character varying(255) NOT NULL, "author_discord_id" character varying(200) NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "web_reservation_spot_id_6b297c19_fk_web_spot_id" FOREIGN KEY ("spot_id") REFERENCES "public"."web_spot" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "unique_reservation_time_and_space_per_guild" to table: "web_reservation"
CREATE UNIQUE INDEX "unique_reservation_time_and_space_per_guild" ON "public"."web_reservation" ("start_at", "end_at", "spot_id", "guild_id");
-- Create index "web_reservation_spot_id_6b297c19" to table: "web_reservation"
CREATE INDEX "web_reservation_spot_id_6b297c19" ON "public"."web_reservation" ("spot_id");
-- Create index "web_reservations_no_overlapping_ranges" to table: "web_reservation"
CREATE INDEX "web_reservations_no_overlapping_ranges" ON "public"."web_reservation" USING gist ("spot_id", "guild_id", (tstzrange(start_at, end_at)));
