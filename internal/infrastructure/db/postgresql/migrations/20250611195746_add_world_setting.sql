
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
-- Drop index "web_reservations_no_overlapping_ranges" from table: "web_reservation"
DROP INDEX "public"."web_reservations_no_overlapping_ranges";
-- Modify "web_reservation" table
ALTER TABLE "public"."web_reservation" ADD CONSTRAINT "unique_reservation_time_and_space_per_guild" UNIQUE USING INDEX "unique_reservation_time_and_space_per_guild", ADD CONSTRAINT "web_reservations_no_overlapping_ranges" EXCLUDE USING gist ("spot_id" WITH =, "guild_id" WITH =, (tstzrange(start_at, end_at)) WITH &&), ALTER CONSTRAINT "web_reservation_spot_id_6b297c19_fk_web_spot_id" DEFERRABLE INITIALLY DEFERRED;


