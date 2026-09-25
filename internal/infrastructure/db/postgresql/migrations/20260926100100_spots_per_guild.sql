ALTER TABLE web_spot ADD COLUMN guild_id varchar(255), ADD COLUMN archived_at timestamptz;

-- Every existing spot belongs to Celesta Community (DECISIONS #3).
UPDATE web_spot SET guild_id = '806152499760201738' WHERE guild_id IS NULL;

-- Keep the oldest spot of each case-insensitive duplicate name active, archive the rest (DECISIONS #28).
UPDATE web_spot s SET archived_at = now()
WHERE EXISTS (
  SELECT 1 FROM web_spot o
  WHERE o.guild_id = s.guild_id
    AND lower(o.name) = lower(s.name)
    AND o.id < s.id
    AND o.archived_at IS NULL
);

CREATE UNIQUE INDEX web_spot_guild_active_name_uidx ON web_spot (guild_id, lower(name)) WHERE archived_at IS NULL;
CREATE INDEX web_reservation_guild_start_idx ON web_reservation (guild_id, start_at);
CREATE INDEX web_reservation_guild_spot_start_idx ON web_reservation (guild_id, spot_id, start_at);
CREATE INDEX web_reservation_author_trgm_idx ON web_reservation USING gin (lower(author) gin_trgm_ops);
