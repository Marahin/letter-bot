-- Modify "guilds" table
ALTER TABLE "public"."guilds" ADD COLUMN "created_at" timestamptz NOT NULL DEFAULT now();
-- Create "roles" table
CREATE TABLE "public"."roles" ("role_id" character varying(255) NOT NULL, "role_name" character varying(255) NOT NULL, "guild_id" character varying(255) NOT NULL, "created_at" timestamp NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY ("role_id"), CONSTRAINT "roles_guild_id_fkey" FOREIGN KEY ("guild_id") REFERENCES "public"."guilds" ("guild_id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create "users" table
CREATE TABLE "public"."users" ("discord_id" character varying(255) NOT NULL, "username" character varying(255) NOT NULL, "avatar_url" character varying(255) NULL, "created_at" timestamp NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY ("discord_id"));
-- Create "sessions" table
CREATE TABLE "public"."sessions" ("session_id" serial NOT NULL, "session_cookie" character varying(255) NOT NULL, "discord_id" character varying(255) NOT NULL, "expires_at" timestamp NOT NULL, "created_at" timestamp NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY ("session_id"), CONSTRAINT "sessions_discord_id_fkey" FOREIGN KEY ("discord_id") REFERENCES "public"."users" ("discord_id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create "user_roles" table
CREATE TABLE "public"."user_roles" ("user_role_id" serial NOT NULL, "discord_id" character varying(255) NOT NULL, "guild_id" character varying(255) NOT NULL, "role_id" character varying(255) NOT NULL, "created_at" timestamp NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY ("user_role_id"), CONSTRAINT "user_roles_discord_id_fkey" FOREIGN KEY ("discord_id") REFERENCES "public"."users" ("discord_id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "user_roles_guild_id_fkey" FOREIGN KEY ("guild_id") REFERENCES "public"."guilds" ("guild_id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "user_roles_role_id_fkey" FOREIGN KEY ("role_id") REFERENCES "public"."roles" ("role_id") ON UPDATE NO ACTION ON DELETE CASCADE);
