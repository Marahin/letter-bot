-- Create "django_migrations" table
CREATE TABLE "public"."django_migrations" ("id" bigserial NOT NULL, "app" character varying(255) NOT NULL, "name" character varying(255) NOT NULL, "applied" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create "django_session" table
CREATE TABLE "public"."django_session" ("session_key" character varying(40) NOT NULL, "session_data" text NOT NULL, "expire_date" timestamptz NOT NULL, PRIMARY KEY ("session_key"));
-- Create index "django_session_expire_date_a5c62663" to table: "django_session"
CREATE INDEX "django_session_expire_date_a5c62663" ON "public"."django_session" ("expire_date");
-- Create index "django_session_session_key_c0390e0f_like" to table: "django_session"
CREATE INDEX "django_session_session_key_c0390e0f_like" ON "public"."django_session" ("session_key" varchar_pattern_ops);
-- Create "django_content_type" table
CREATE TABLE "public"."django_content_type" ("id" serial NOT NULL, "app_label" character varying(100) NOT NULL, "model" character varying(100) NOT NULL, PRIMARY KEY ("id"));
-- Create index "django_content_type_app_label_model_76bd3d3b_uniq" to table: "django_content_type"
CREATE UNIQUE INDEX "django_content_type_app_label_model_76bd3d3b_uniq" ON "public"."django_content_type" ("app_label", "model");
-- Create "auth_permission" table
CREATE TABLE "public"."auth_permission" ("id" serial NOT NULL, "name" character varying(255) NOT NULL, "content_type_id" integer NOT NULL, "codename" character varying(100) NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "auth_permission_content_type_id_2f476e4b_fk_django_co" FOREIGN KEY ("content_type_id") REFERENCES "public"."django_content_type" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "auth_permission_content_type_id_2f476e4b" to table: "auth_permission"
CREATE INDEX "auth_permission_content_type_id_2f476e4b" ON "public"."auth_permission" ("content_type_id");
-- Create index "auth_permission_content_type_id_codename_01ab375a_uniq" to table: "auth_permission"
CREATE UNIQUE INDEX "auth_permission_content_type_id_codename_01ab375a_uniq" ON "public"."auth_permission" ("content_type_id", "codename");
-- Create "auth_group" table
CREATE TABLE "public"."auth_group" ("id" serial NOT NULL, "name" character varying(150) NOT NULL, PRIMARY KEY ("id"));
-- Create index "auth_group_name_a6ea08ec_like" to table: "auth_group"
CREATE INDEX "auth_group_name_a6ea08ec_like" ON "public"."auth_group" ("name" varchar_pattern_ops);
-- Create index "auth_group_name_key" to table: "auth_group"
CREATE UNIQUE INDEX "auth_group_name_key" ON "public"."auth_group" ("name");
-- Create "auth_group_permissions" table
CREATE TABLE "public"."auth_group_permissions" ("id" bigserial NOT NULL, "group_id" integer NOT NULL, "permission_id" integer NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "auth_group_permissio_permission_id_84c5c92e_fk_auth_perm" FOREIGN KEY ("permission_id") REFERENCES "public"."auth_permission" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "auth_group_permissions_group_id_b120cbf9_fk_auth_group_id" FOREIGN KEY ("group_id") REFERENCES "public"."auth_group" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "auth_group_permissions_group_id_b120cbf9" to table: "auth_group_permissions"
CREATE INDEX "auth_group_permissions_group_id_b120cbf9" ON "public"."auth_group_permissions" ("group_id");
-- Create index "auth_group_permissions_group_id_permission_id_0cd325b0_uniq" to table: "auth_group_permissions"
CREATE UNIQUE INDEX "auth_group_permissions_group_id_permission_id_0cd325b0_uniq" ON "public"."auth_group_permissions" ("group_id", "permission_id");
-- Create index "auth_group_permissions_permission_id_84c5c92e" to table: "auth_group_permissions"
CREATE INDEX "auth_group_permissions_permission_id_84c5c92e" ON "public"."auth_group_permissions" ("permission_id");
-- Create "auth_user" table
CREATE TABLE "public"."auth_user" ("id" serial NOT NULL, "password" character varying(128) NOT NULL, "last_login" timestamptz NULL, "is_superuser" boolean NOT NULL, "username" character varying(150) NOT NULL, "first_name" character varying(150) NOT NULL, "last_name" character varying(150) NOT NULL, "email" character varying(254) NOT NULL, "is_staff" boolean NOT NULL, "is_active" boolean NOT NULL, "date_joined" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create index "auth_user_username_6821ab7c_like" to table: "auth_user"
CREATE INDEX "auth_user_username_6821ab7c_like" ON "public"."auth_user" ("username" varchar_pattern_ops);
-- Create index "auth_user_username_key" to table: "auth_user"
CREATE UNIQUE INDEX "auth_user_username_key" ON "public"."auth_user" ("username");
-- Create "auth_user_groups" table
CREATE TABLE "public"."auth_user_groups" ("id" bigserial NOT NULL, "user_id" integer NOT NULL, "group_id" integer NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "auth_user_groups_group_id_97559544_fk_auth_group_id" FOREIGN KEY ("group_id") REFERENCES "public"."auth_group" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "auth_user_groups_user_id_6a12ed8b_fk_auth_user_id" FOREIGN KEY ("user_id") REFERENCES "public"."auth_user" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "auth_user_groups_group_id_97559544" to table: "auth_user_groups"
CREATE INDEX "auth_user_groups_group_id_97559544" ON "public"."auth_user_groups" ("group_id");
-- Create index "auth_user_groups_user_id_6a12ed8b" to table: "auth_user_groups"
CREATE INDEX "auth_user_groups_user_id_6a12ed8b" ON "public"."auth_user_groups" ("user_id");
-- Create index "auth_user_groups_user_id_group_id_94350c0c_uniq" to table: "auth_user_groups"
CREATE UNIQUE INDEX "auth_user_groups_user_id_group_id_94350c0c_uniq" ON "public"."auth_user_groups" ("user_id", "group_id");
-- Create "auth_user_user_permissions" table
CREATE TABLE "public"."auth_user_user_permissions" ("id" bigserial NOT NULL, "user_id" integer NOT NULL, "permission_id" integer NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "auth_user_user_permi_permission_id_1fbb5f2c_fk_auth_perm" FOREIGN KEY ("permission_id") REFERENCES "public"."auth_permission" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "auth_user_user_permissions_user_id_a95ead1b_fk_auth_user_id" FOREIGN KEY ("user_id") REFERENCES "public"."auth_user" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- Create index "auth_user_user_permissions_permission_id_1fbb5f2c" to table: "auth_user_user_permissions"
CREATE INDEX "auth_user_user_permissions_permission_id_1fbb5f2c" ON "public"."auth_user_user_permissions" ("permission_id");
-- Create index "auth_user_user_permissions_user_id_a95ead1b" to table: "auth_user_user_permissions"
CREATE INDEX "auth_user_user_permissions_user_id_a95ead1b" ON "public"."auth_user_user_permissions" ("user_id");
-- Create index "auth_user_user_permissions_user_id_permission_id_14a6b632_uniq" to table: "auth_user_user_permissions"
CREATE UNIQUE INDEX "auth_user_user_permissions_user_id_permission_id_14a6b632_uniq" ON "public"."auth_user_user_permissions" ("user_id", "permission_id");
-- Create "django_admin_log" table
CREATE TABLE "public"."django_admin_log" ("id" serial NOT NULL, "action_time" timestamptz NOT NULL, "object_id" text NULL, "object_repr" character varying(200) NOT NULL, "action_flag" smallint NOT NULL, "change_message" text NOT NULL, "content_type_id" integer NULL, "user_id" integer NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "django_admin_log_content_type_id_c4bce8eb_fk_django_co" FOREIGN KEY ("content_type_id") REFERENCES "public"."django_content_type" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "django_admin_log_user_id_c564eba6_fk_auth_user_id" FOREIGN KEY ("user_id") REFERENCES "public"."auth_user" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "django_admin_log_action_flag_check" CHECK (action_flag >= 0));
-- Create index "django_admin_log_content_type_id_c4bce8eb" to table: "django_admin_log"
CREATE INDEX "django_admin_log_content_type_id_c4bce8eb" ON "public"."django_admin_log" ("content_type_id");
-- Create index "django_admin_log_user_id_c564eba6" to table: "django_admin_log"
CREATE INDEX "django_admin_log_user_id_c564eba6" ON "public"."django_admin_log" ("user_id");
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
-- Modify "django_content_type" table
ALTER TABLE "public"."django_content_type" ADD CONSTRAINT "django_content_type_app_label_model_76bd3d3b_uniq" UNIQUE USING INDEX "django_content_type_app_label_model_76bd3d3b_uniq";

