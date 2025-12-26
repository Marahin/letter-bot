-- Enable trigram extension for efficient text search
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- web_reservation indexes
CREATE INDEX IF NOT EXISTS web_reservation_guild_author_end_idx
ON web_reservation (guild_id, author_discord_id, end_at ASC);

-- web_spot indexes
-- Standard B-Tree for equality checks (=)
CREATE INDEX IF NOT EXISTS web_spot_lower_name_idx
ON web_spot (lower(name));

-- GIN Trigram index for pattern matching (LIKE %...%)
CREATE INDEX IF NOT EXISTS web_spot_lower_name_trgm_idx
ON web_spot USING GIN (lower(name) gin_trgm_ops);

-- Optimizing listing of all upcoming reservations for a guild
CREATE INDEX IF NOT EXISTS web_reservation_guild_end_idx
ON web_reservation (guild_id, end_at ASC);
