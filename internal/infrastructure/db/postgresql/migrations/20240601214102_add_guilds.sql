-- Modify "auth_group" table
ALTER TABLE "public"."auth_group" ADD CONSTRAINT "auth_group_name_key" UNIQUE USING INDEX "auth_group_name_key";
-- Modify "auth_group_permissions" table
ALTER TABLE "public"."auth_group_permissions" ADD CONSTRAINT "auth_group_permissions_group_id_permission_id_0cd325b0_uniq" UNIQUE USING INDEX "auth_group_permissions_group_id_permission_id_0cd325b0_uniq";
-- Modify "auth_permission" table
ALTER TABLE "public"."auth_permission" ADD CONSTRAINT "auth_permission_content_type_id_codename_01ab375a_uniq" UNIQUE USING INDEX "auth_permission_content_type_id_codename_01ab375a_uniq";
-- Modify "auth_user" table
ALTER TABLE "public"."auth_user" ADD CONSTRAINT "auth_user_username_key" UNIQUE USING INDEX "auth_user_username_key";
-- Modify "auth_user_groups" table
ALTER TABLE "public"."auth_user_groups" ADD CONSTRAINT "auth_user_groups_user_id_group_id_94350c0c_uniq" UNIQUE USING INDEX "auth_user_groups_user_id_group_id_94350c0c_uniq";
-- Modify "auth_user_user_permissions" table
ALTER TABLE "public"."auth_user_user_permissions" ADD CONSTRAINT "auth_user_user_permissions_user_id_permission_id_14a6b632_uniq" UNIQUE USING INDEX "auth_user_user_permissions_user_id_permission_id_14a6b632_uniq";
-- Modify "django_content_type" table
ALTER TABLE "public"."django_content_type" ADD CONSTRAINT "django_content_type_app_label_model_76bd3d3b_uniq" UNIQUE USING INDEX "django_content_type_app_label_model_76bd3d3b_uniq";
-- Drop index "web_reservations_no_overlapping_ranges" from table: "web_reservation"
DROP INDEX "public"."web_reservations_no_overlapping_ranges";
-- Modify "web_reservation" table
ALTER TABLE "public"."web_reservation" ADD CONSTRAINT "unique_reservation_time_and_space_per_guild" UNIQUE USING INDEX "unique_reservation_time_and_space_per_guild", ADD CONSTRAINT "web_reservations_no_overlapping_ranges" EXCLUDE USING gist ("spot_id" WITH =, "guild_id" WITH =, (tstzrange(start_at, end_at)) WITH &&);
-- Create "guilds" table
CREATE TABLE "public"."guilds" ("id" serial NOT NULL, "guild_id" character varying(255) NOT NULL, "guild_name" character varying(255) NOT NULL, PRIMARY KEY ("id"));
-- Create index "guilds_guild_id_uindex" to table: "guilds"
CREATE UNIQUE INDEX "guilds_guild_id_uindex" ON "public"."guilds" ("guild_id");
