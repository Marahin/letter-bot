-- Modify "django_content_type" table
ALTER TABLE "public"."django_content_type" ADD CONSTRAINT "django_content_type_app_label_model_76bd3d3b_uniq" UNIQUE USING INDEX "django_content_type_app_label_model_76bd3d3b_uniq";
-- Create "guilds_world" table
CREATE TABLE "public"."guilds_world" (
  "id" bigserial NOT NULL,
  "guild_id" character varying(255) NOT NULL,
  "world_name" character varying(100) NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "guilds_world_guild_id_key" UNIQUE ("guild_id")
);
-- Modify "auth_permission" table
ALTER TABLE "public"."auth_permission" ADD CONSTRAINT "auth_permission_content_type_id_codename_01ab375a_uniq" UNIQUE USING INDEX "auth_permission_content_type_id_codename_01ab375a_uniq", ALTER CONSTRAINT "auth_permission_content_type_id_2f476e4b_fk_django_co" DEFERRABLE INITIALLY DEFERRED;
-- Modify "auth_group" table
ALTER TABLE "public"."auth_group" ADD CONSTRAINT "auth_group_name_key" UNIQUE USING INDEX "auth_group_name_key";
-- Modify "auth_group_permissions" table
ALTER TABLE "public"."auth_group_permissions" ADD CONSTRAINT "auth_group_permissions_group_id_permission_id_0cd325b0_uniq" UNIQUE USING INDEX "auth_group_permissions_group_id_permission_id_0cd325b0_uniq", ALTER CONSTRAINT "auth_group_permissio_permission_id_84c5c92e_fk_auth_perm" DEFERRABLE INITIALLY DEFERRED, ALTER CONSTRAINT "auth_group_permissions_group_id_b120cbf9_fk_auth_group_id" DEFERRABLE INITIALLY DEFERRED;
-- Modify "auth_user" table
ALTER TABLE "public"."auth_user" ADD CONSTRAINT "auth_user_username_key" UNIQUE USING INDEX "auth_user_username_key";
-- Modify "auth_user_groups" table
ALTER TABLE "public"."auth_user_groups" ADD CONSTRAINT "auth_user_groups_user_id_group_id_94350c0c_uniq" UNIQUE USING INDEX "auth_user_groups_user_id_group_id_94350c0c_uniq", ALTER CONSTRAINT "auth_user_groups_group_id_97559544_fk_auth_group_id" DEFERRABLE INITIALLY DEFERRED, ALTER CONSTRAINT "auth_user_groups_user_id_6a12ed8b_fk_auth_user_id" DEFERRABLE INITIALLY DEFERRED;
-- Modify "auth_user_user_permissions" table
ALTER TABLE "public"."auth_user_user_permissions" ADD CONSTRAINT "auth_user_user_permissions_user_id_permission_id_14a6b632_uniq" UNIQUE USING INDEX "auth_user_user_permissions_user_id_permission_id_14a6b632_uniq", ALTER CONSTRAINT "auth_user_user_permi_permission_id_1fbb5f2c_fk_auth_perm" DEFERRABLE INITIALLY DEFERRED, ALTER CONSTRAINT "auth_user_user_permissions_user_id_a95ead1b_fk_auth_user_id" DEFERRABLE INITIALLY DEFERRED;
-- Modify "django_admin_log" table
ALTER TABLE "public"."django_admin_log" ALTER CONSTRAINT "django_admin_log_content_type_id_c4bce8eb_fk_django_co" DEFERRABLE INITIALLY DEFERRED, ALTER CONSTRAINT "django_admin_log_user_id_c564eba6_fk_auth_user_id" DEFERRABLE INITIALLY DEFERRED;
-- Drop index "web_reservations_no_overlapping_ranges" from table: "web_reservation"
DROP INDEX "public"."web_reservations_no_overlapping_ranges";
-- Modify "web_reservation" table
ALTER TABLE "public"."web_reservation" ADD CONSTRAINT "unique_reservation_time_and_space_per_guild" UNIQUE USING INDEX "unique_reservation_time_and_space_per_guild", ADD CONSTRAINT "web_reservations_no_overlapping_ranges" EXCLUDE USING gist ("spot_id" WITH =, "guild_id" WITH =, (tstzrange(start_at, end_at)) WITH &&), ALTER CONSTRAINT "web_reservation_spot_id_6b297c19_fk_web_spot_id" DEFERRABLE INITIALLY DEFERRED;
