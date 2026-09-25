# Letter web panel: decisions for manual review

Each item is a decision taken without the owner. Review and reverse as needed.

## Scope and data

1. **Same repository, new binary.** The web lives in this repository as `cmd/web`. It shares the PostgreSQL database with the bot. The bot and the web are separate processes.
2. **The Django admin is superseded.** `spot-assistant-web` (Django, `spot-assistant-web.tibialoot.com`) is not changed. All schema changes are additive, so Django keeps working. Decommission it after the Letter web is live.
3. **Spots become per guild.** `web_spot` gets a nullable `guild_id` and `archived_at`. A migration assigns every existing spot to Celesta Community (`806152499760201738`). A new guild starts with no spots. The web offers "Import the default spot list" (names from `seeds/spots.sql`).
4. **Delete a spot = archive it.** A spot with reservations is archived, not deleted, so history and stats stay correct. A spot without reservations is deleted.
5. **Celesta Community is premium forever.** A migration inserts guild `806152499760201738` with premium forever. Premium is only set in the database (no payments, no self-service). An instance admin (`WEB_ADMIN_DISCORD_IDS`) can toggle premium in the web.
6. **Non-premium guild = inactive bot.** The bot answers every command in a non-premium guild with a short "this server is not premium" reply and posts no summary.

## Experience and player data

7. **TibiaData is the only player data source.** The web uses the in-cluster `ext-tibiadata-api` (`TIBIA_WORLD_API_BASE_URL`). The stale `tibiadata-postgres` data is not used.
8. **Experience comes from highscore snapshots.** Every 15 minutes a job reads all 20 experience highscore pages for each world in `guilds_world`. The job stores one snapshot per character.
9. **Experience per reservation.** After a reservation ends, the job computes the gain for each character in `author` ("/"-separated). Gain = the first snapshot at or after `end_at` minus the last snapshot at or before `start_at`. The precision is the highscore refresh interval (about 15 minutes).
10. **Only the top 1000 are tracked.** A character outside the world top 1000 has no snapshots. Stats show "no data" for that character.
11. **No backfill.** Experience stats start when the job first runs. Reservation counts and hours cover the full history.
