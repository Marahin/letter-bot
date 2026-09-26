# Letter web panel

The web panel is the `cmd/web` binary (`letter-web`). It shares the PostgreSQL
database with the bot.

## Discord application

The web uses the OAuth2 settings of the same Discord application as the bot.

1. Open the Discord developer portal, then the application, then **OAuth2**.
2. Add this redirect URL: `<WEB_BASE_URL>/auth/callback`. For example,
   `https://letter.tibialoot.com/auth/callback`. For local work, add
   `http://localhost:8080/auth/callback`.
3. Copy the client ID and the client secret into `DISCORD_CLIENT_ID` and
   `DISCORD_CLIENT_SECRET`. The web does not start without them.

The web asks for these OAuth scopes: `identify guilds guilds.members.read`.
`guilds.members.read` gives the roles of the signed-in user in a server. The bot
therefore needs no privileged intent (decision 14).

The web stores the OAuth tokens in `web_users` and refreshes an access token
shortly before it expires. When Discord refuses a token, the web signs the user
out and sends them to the login page.

## Access

- A user sees a server when the bot is in it and the user may view it: owner or
  Administrator, a manage, view or reserve rank, or any member when the server
  has no reserve rank (decision 23).
- A server the user may not view answers 404, never 403.
- Site admins (`WEB_ADMIN_DISCORD_IDS`, comma-separated Discord user ids) see
  every server the bot is in, with full rights. They also see **Admin > Servers**
  (`/admin/guilds`), where they turn premium on and off. The admin routes answer
  404 to everyone else.
- On a server without premium, the feature pages show "Premium required".
  Settings and Channels work without premium, so an admin can prepare the server.

## Settings and Channels

Only the server owner and administrators open these pages.

- **Settings** (`/servers/{id}/settings`):
  - Re-invite the bot, to apply missing permissions.
  - **Refresh server data** sets the durable resync flag and sends
    `letter_guild_resync`. The bot then copies the channels and roles again. One
    request per server every 5 minutes (kept in the memory of the web process).
  - **Tibia world**: the same value as `/world-set`. Saving sends `letter_guild_config`.
  - Four rank lists: manage, view, reserve, overbook. With no reserve rank, everyone
    can reserve. With no overbook rank, the `Postman` role can overbook. The lists
    apply on the next request or command, so no signal is sent.
- **Channels** (`/servers/{id}/channels`): the command channel (empty = `/book`
  and `/unbook` work in every channel) and the summary channel (empty =
  `#letter-summary`). Saving sends `letter_guild_config` and `letter_summary_refresh`.
- The pickers list only the channels and roles that the bot copied. A value that
  is not in that list is refused.

## Respawns

`/servers/{id}/spots` needs premium. Members with view access see the list.
Managers can change it.

- **Add** and **rename**: the name is trimmed and has 1 to 120 characters. Names
  are unique among the active respawns of the server, not case-sensitive.
- **Remove**: a respawn that no reservation points at is deleted. Any other
  respawn is archived, so its history and stats stay. The button says which one
  happens ("Delete" or "Archive"), and the confirm dialog says why.
- **Restore** moves an archived respawn back to the active list. It is refused
  when an active respawn has the same name.
- **Import the default list** shows only on a server without respawns. It adds
  the 208 names from `seeds/spots.sql` and skips names that exist.
- Every change sends `letter_summary_refresh`.
- The upcoming count (and the name, for members who cannot rename) links to the respawn's own page,
  `/servers/{id}/spots/{spot}`: its reservations and counts.

## Reservations

`/servers/{id}/reservations` needs premium. Members with view access see every
reservation of the server. The filter bar narrows the list by respawn, author
(part of the text, not case-sensitive), day range (From and To are whole days),
time (Upcoming, the default, Past or All) and "Only mine". Each change updates
the list and the address, so a filtered list can be shared. A page has 50 rows.

The web uses the rules of `/book`:

- A reservation takes up to 3 hours. One author can book 3 hours within 24
  hours (the same multi-floor rule as the bot). An author typed as free text
  (no Discord account) has no quota.
- **New reservation** (reserve access): a member books as themselves (their
  server nick). A manager can choose a member who booked before (type 2 or more
  letters) or type any author. The start cannot be in the past.
- **Overbook** shows only to members who can overbook (managers, the overbook
  ranks, or `@Postman` when no overbook rank is set). Overbooked authors get the
  bot's Discord message (`letter_overbooked`). The bot's "abandoned reservation"
  rule applies too.
- **Edit**: the author of a reservation and managers can change the respawn and
  the times until the reservation ends. Only managers change the author. An edit
  never overbooks: an overlap is refused and the overlapping reservations are
  listed. An ongoing reservation keeps its start.
- **Delete**: the author before the reservation ends, managers at any time.
- Every change sends `letter_summary_refresh`.
- Times are read and shown in the server time zone (`TZ`, Europe/Berlin).

## Experience job

The web process reads the TibiaData experience highscores of each world set by a
premium server (`TIBIA_WORLD_API_BASE_URL`, pages 1 to 20 = the world top 1000).
It runs at start and then every `WEB_EXPERIENCE_JOB_INTERVAL` (15 minutes). Only
one web process runs it at a time (Postgres advisory lock `7419001`). Set
`WEB_EXPERIENCE_JOB_ENABLED=false` to stop it.

- **Tracked characters** are the characters in the author of a reservation
  (split on `/`) that is upcoming, active or ended in the last 24 hours. The job
  does not call TibiaData for a world with no tracked character.
- **Snapshots** (`highscore_snapshots`): a new row only when the level or the
  experience of a tracked character changed. A run that sees the same value
  moves `last_seen_at` of the latest row. A row therefore says "this value was
  seen from `observed_at` to `last_seen_at`".
- **Runs** (`highscore_runs`): one row per complete read. `observed_at` is the
  TibiaData scrape time (`information.timestamp`, or the request time) minus
  `highscore_age` minutes, the oldest of all pages. A failed page writes no run.
- **Gain per reservation** (`reservation_experience`, one row per character),
  computed once the reservation has ended and a run was observed at or after
  `end_at`:
  - start = the latest snapshot at or before `start_at`, if it was still seen
    at most 2 hours before `start_at`. Otherwise the first snapshot within 20
    minutes after `start_at`.
  - end = the latest snapshot at or before that first run after `end_at`, if it
    was seen at or after `end_at`.
  - gain = end - start (negative after a death). A missing start or end gives
    `status = no_data`: the character was outside the top 1000, or was not
    tracked when the job ran.
- Only reservations that start after the first run of the world, and that ended
  in the last 48 hours, get a gain. There is no backfill.

To test the job against a real database, apply the migrations and run
`LETTER_TEST_DATABASE_URL=postgres://… go test -run 'EndToEnd|PgAdvisoryLock' ./internal/infrastructure/experience/postgresql/sqlc/`.
The test uses the guild ids `it-exp-*` and the worlds `Itworld*`, and deletes them after.

## Stats

Every Stats page needs the view rank and a premium server. Pages:
`/servers/{id}/stats` (overview), `/stats/spots`, `/stats/players`,
`/stats/characters` (tables), `/stats/spots/{spot}`, `/stats/players/{user}`
and `/servers/{id}/characters/{name}` (the character page).

- **Day range.** The shared range picker (`web.RangePicker`) sets the days. The
  page uses the span from the first to the last picked day (picked single days
  do not make gaps), at most 400 days, default the last 30 days. Days are local
  days of the process time zone (`TZ`, decision 31); a reservation belongs to
  the day on which it starts. The span is stored per server in the
  `letter_range_{id}` session cookie as its first and last day.
- **Figures.** Reservations and booked hours (the reservation length, not play
  time, decision 36) count every reservation that starts in the range.
  Experience is the sum of the `ok` gains in `reservation_experience`.
  Experience per hour divides it by the hours of the reservations that have at
  least one `ok` gain. On a character's rows only that character's own gain
  counts; on a respawn or player row every character of the party counts.
- **No data.** A figure without any `ok` gain shows "No data" with a tooltip and
  a note on the page, never 0: the highscores show only the top 1000 of a world.
  The daily experience chart plots only the days that have data. A reservation
  that has not been attributed yet shows "Pending" on the character page.
- **Players** are Discord users (`author_discord_id`). Their name is the author
  text of their latest reservation. Reservations with a free-text author (made
  by a manager) are not in the player table; they are in the respawn and
  character tables.
- **Characters** come from the author text split on `/` and compared by
  `lower(trim(name))`.
- **Tables** sort on the server (`?sort=name|reservations|hours|exp|exp_h&dir=asc|desc`);
  a row without experience data sorts last in both directions. A table shows at
  most 500 rows; `?format=csv` exports every row (an empty cell means no data).
- **Leaderboards** on the overview: the 10 players with the most booked hours,
  the 10 characters with the most experience, and the 10 characters with the
  best experience per hour among those with at least 3 hours of data.
- **Character page.** The TibiaData profile (`/v4/character/{name}`, cached 5
  minutes in the web process, including "not found") and the experience history
  from `highscore_snapshots` of the server's world (the last value of each day).
  When TibiaData fails, the page shows a notice and the statistics. A name that
  TibiaData and the server's reservations do not know answers 404.

Query cost, measured on 430 000 generated reservations of one guild (the size of
Celesta) with 141 000 experience rows, PostgreSQL 12: a 30-day overview (four
aggregate queries plus the picker days) takes about 230 ms; a 400-day overview
about 0.6 s, the character totals being the largest part (0.85 s). One respawn,
player or character takes 1-30 ms. The existing indexes are enough
(`web_reservation_guild_start_idx`, `web_reservation_guild_spot_start_idx`,
`web_reservation_guild_author_end_idx`, the author trigram index and the
`reservation_experience` primary key), so there is no new migration.

To test the stats queries against a real database, run
`LETTER_TEST_DATABASE_URL=postgres://… go test -run TestStatsRepository_Queries ./internal/infrastructure/stats/postgresql/sqlc/`.
The test uses the guild id `it-stats-guild` and deletes its rows after.

## Loot Calculator

`/tools/loot-calculator` is public. It uses the top bar when you are signed out
and the sidebar when you are signed in. The logic is a port of the tibialoot.com
calculator (`tibiadata-front/components/Calculator.vue`) in
`internal/core/lootcalc`:

- A player block is a name line without indentation, followed by indented
  `Key: value` lines. Each block must have Loot, Supplies, Balance, Damage and
  Healing. The parser accepts CRLF, tabs, comma thousands and negative values,
  and removes ` (Leader)` from the name. It ignores unknown keys.
- The share per player is the total balance divided by the number of players,
  rounded down (also for a negative total). Each player above the share gives,
  the largest amount first. Each player at or below the share receives, in paste
  order. Up to (players - 1) gp stays with the players who give.
- The Discord and TeamSpeak texts are the same as in the original. The one
  change: the TeamSpeak summary prints "gp" once, not twice.

The server does the calculation and stores nothing. The browser keeps the last
20 sessions (with the pasted text) in `localStorage` (`letter:loot-history`).
`dist/loot-calculator.js` does the copy buttons (the Clipboard API, with an
`execCommand` fallback on plain HTTP), the history and the Load button (it posts
the saved text again). The form limit is 64 KB.
