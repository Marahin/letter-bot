# TibiaLoot.com web panel (`letter-web`)

The web panel is the `cmd/web` binary (`letter-web`), served as TibiaLoot.com.
The Discord bot keeps the name Letter. The panel shares the PostgreSQL database
with the bot.

## Discord application

The web uses the OAuth2 settings of the same Discord application as the bot.

1. Open the Discord developer portal, then the application, then **OAuth2**.
2. Add this redirect URL: `<WEB_BASE_URL>/auth/callback`. For example,
   `https://tibialoot.com/auth/callback`. For local work, add
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
  every stored server, with full rights, including the ones the bot has left
  (listed after the others). They also see **Admin > Servers**
  (`/admin/guilds`), where they turn premium on and off. The admin routes answer
  404 to everyone else.
- On a server without premium, the feature pages show "Premium required".
  Settings and Channels work without premium, so an admin can prepare the server.
- The server a user opens last is their default (`web_users.default_guild_id`),
  so the sidebar keeps it selected on every page and after the next sign-in. It
  is written only when it changes. Without a default (or when the user lost
  access to it), the sidebar picks the first premium server, else the first.

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
  server nick). A manager can choose a member who booked before (type 3 or more
  letters) or type any author. The start cannot be in the past.
- **Overbook** shows only to members who can overbook (managers, the overbook
  ranks, or `@Postman` when no overbook rank is set). Overbooked authors get the
  bot's Discord message (`letter_overbooked`). The bot's "abandoned reservation"
  rule applies too.
- **Edit**: the author of a reservation and managers can change the respawn and
  the times until the reservation ends. Only managers change the author. An edit
  never overbooks: an overlap is refused and the overlapping reservations are
  listed. An ongoing reservation keeps its start and its respawn.
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
  `highscore_age` minutes, the oldest of all pages. A failed page writes no run;
  so does an empty page or a page before the last one with fewer than 50 rows
  (a partial TibiaData answer would make tracked characters look absent).
- **Gain per reservation** (`reservation_experience`, one row per character),
  computed once the reservation has ended and a run was observed at or after
  `end_at`:
  - start = the latest snapshot at or before `start_at`, if it was still seen
    at most 2 hours before `start_at`. Otherwise the first snapshot within 20
    minutes after `start_at`.
  - end = the latest snapshot at or before that first run after `end_at`, if it
    was seen at or after `end_at`. When that run did not see the character, the
    next run stands in if it was observed within two job intervals; until the
    next run exists, the reservation waits.
  - gain = end - start (negative after a death). A missing start or end gives
    `status = no_data`: the character was outside the top 1000, or was not
    tracked when the job ran.
- Only reservations that start after the first run of the world, and that ended
  in the last 48 hours, get a gain. There is no backfill.

To test the job against a real database (the test applies the migrations), run
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
  a row without experience data sorts last in both directions. Sorting and the
  row cap run in SQL: a table shows at most 500 rows (a detail page table 25);
  `?format=csv` exports every row (an empty cell means no data; a name that
  starts with `= + - @`, a tab or a carriage return gets a leading `'` so a
  spreadsheet does not run it as a formula).
- **Leaderboards** on the overview: the 10 players with the most booked hours,
  the 10 characters with the most experience, and the 10 characters with the
  best experience per hour among those with at least 3 hours of data.
- **Character page.** The TibiaData profile (`/v4/character/{name}`, cached 5
  minutes in the web process, including "not found"; a TibiaData failure is
  cached 30 seconds and a lookup waits at most 5 seconds) and the experience history
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

`/tools/loot-calculator` is public, and the landing page (`/`) opens with the
same calculator ready to use (`toolshttp.LandingCalculator`, handed to the shell
by `Server.WithLandingTool`). It uses the top bar when you are signed out and the
sidebar when you are signed in. The logic is a port of the tibialoot.com
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

## Configuration

The web reads these environment variables (`.env.sample` has examples):

| Variable | Required | Default | Use |
|---|---|---|---|
| `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME` | yes | | The shared PostgreSQL database (the same values as the bot). |
| `DATABASE_SSL` | no | `disable` | The libpq `sslmode`, for example `require`. The bot reads it too. |
| `WEB_BASE_URL` | yes | | The public address, for example `https://tibialoot.com`. It makes the OAuth redirect URL. With `https`, the session cookie is `Secure`. |
| `DISCORD_CLIENT_ID`, `DISCORD_CLIENT_SECRET` | yes | | The OAuth2 credentials of the bot's Discord application. The web does not start without them. |
| `WEB_ADDR` | no | `:8080` | The address of the web server. |
| `WEB_METRICS_ADDR` | no | `:3005` | `/metrics`, `/livez`, `/readyz` (`/readyz` pings the database). |
| `WEB_ADMIN_DISCORD_IDS` | no | | Comma-separated Discord user ids of the site admins. |
| `TIBIA_WORLD_API_BASE_URL` | no | | TibiaData v4, for example `http://ext-tibiadata-api:8080/v4`. Without it, the experience job does not run and the character page shows "TibiaData does not answer". |
| `WEB_EXPERIENCE_JOB_ENABLED` | no | `true` | Set `false` to stop the experience job in this process. |
| `WEB_EXPERIENCE_JOB_INTERVAL` | no | `15m` | The time between two job runs. Must be more than 0. |
| `WEB_ASSETS_DIR` | no | | Serve the assets from this directory instead of the embedded copy (development only). |
| `TZ` | yes | | Must be `Europe/Berlin`, the same as the bot (decision 31). The image sets it; the web does not start without it. |

The bot reads one new optional variable: `BOT_WEB_BASE_URL` (or `WEB_BASE_URL`).
It puts this address in its "this server is not premium" reply. Its metrics
address is `BOT_METRICS_ADDR` or `METRICS_ADDR` (default `:2112`), and
`BOT_TOKEN` is required.

Both binaries stop cleanly on SIGINT and SIGTERM. A missing or invalid variable,
an unreachable database or a listen port in use stops the start with exit code 1
and an error in the log.

## Deploy

The production setup is in the Kubernetes namespace `refugees` (context
`default`): `spot-assistant-bot`, the old Django `spot-assistant-web`,
`spot-assistant-postgres` and `ext-tibiadata-api`. The steps below add the Letter
web. Nothing in this branch changes the cluster.

`DATABASE_SSL` now applies to the bot and the web (before, the bot always used
`sslmode=disable`). It must be a valid libpq `sslmode`. The default is `disable`.

### Images

CI builds two images from the one `Dockerfile` and pushes them on `main`:

- `marahin/letter-bot:<sha>` (target `bot`, the default target, entrypoint
  `/spot-assistant-bot`, unchanged).
- `marahin/letter-web:<sha>` (target `web`, entrypoint `/letter-web`, ports
  8080 and 3005).

Locally: `make docker` and `make docker-web`.

### Migrations

Both binaries apply the embedded goose migrations on start, before they open the
pool and serve. A Postgres advisory lock (key 7419000) lets one binary migrate
while the other waits (at most 10 minutes) and then finds nothing pending, so the
bot and the web can start in any order. The history is in `goose_db_version`.

On the first start against the production database, the binary adopts the atlas
history (`atlas_schema_revisions.atlas_schema_revisions`): the versions that
atlas finished are written to `goose_db_version` and do not run again. The start
stops when an atlas revision is partially applied, or when the database has
tables but no history. The atlas schema stays; nothing reads it after the
adoption.

A binary does not serve `/livez` while it migrates. Give each Deployment (the
bot too, on its metrics port) a `startupProbe`, so that the liveness probe does
not kill a pod that migrates or waits for the lock:

```yaml
startupProbe:
  httpGet: { path: /livez, port: metrics }
  periodSeconds: 10
  failureThreshold: 60
```

If a migration hangs, find the holder of the lock and end its session:

```sql
SELECT pg_terminate_backend(pid) FROM pg_locks WHERE locktype = 'advisory' AND objid = 7419000;
```

The interrupted migration rolls back (each file runs in a transaction), and the
next start applies it again.

### Rollout order

**For this release, deploy the bot first.** Migration
`20260926100100_spots_per_guild.sql` adds `web_spot.guild_id`. Three queries of
the bot that runs in production today join `web_spot` and use an unqualified
`guild_id`, which is then ambiguous: `SelectUpcomingMemberReservationsWithSpots`
(the `/unbook` autocomplete), `SelectReservationsWithSpots` (the summary) and
`SelectReservationsWithSpotsForSpot` (the private summary). They fail from the
migration until the new bot runs. From the next release on, the order does not
matter.

1. Take a database backup (`pg_dump` of `spotassistant`).
2. Check the atlas history:
   `SELECT version, type, applied, total, error FROM atlas_schema_revisions.atlas_schema_revisions ORDER BY version;`
   Every row must be finished (`applied = total` and no `error`, or a baseline),
   and the last version must be `20251211123500` or a later file of this
   release. Otherwise, fix the history by hand first; the binaries refuse to
   start on it.
3. Set the new `marahin/letter-bot:<sha>` image on `spot-assistant-bot` (with
   the `startupProbe` above). It migrates on start: five new files,
   `20260926100000` to `20260926100300` and `20261004100000`.
   `20260926100100` builds three indexes on `web_reservation` (about 430k rows)
   without `CONCURRENTLY` (each file runs in a transaction). Each build holds a
   lock that blocks writes to `web_reservation` (reads still work) for a few
   seconds, so bookings can fail or wait during this step. Do it at a quiet
   hour. Expect a break of `/unbook` autocomplete and the summaries of about
   one minute, until the new bot pod is ready.
4. Deploy `letter-web` (below). The bot works without the web.

On start the new bot upserts each guild into `guilds`, copies its channels and
roles, and keeps working for Celesta Community (premium forever, set by
migration `20260926100000`). The only behaviour change for Celesta: owners,
administrators and managers can overbook without the `Postman` role
(decision 23). No manage rank is set at first, so only the owner and the
administrators are managers until an admin sets ranks in Settings.

### The Django admin

The Django admin (`spot-assistant-web`) keeps running against the same tables,
but do not use it to manage spots after the migration:

- A spot that Django creates has `guild_id = NULL`. The bot and the web do not
  see it.
- Django shows all spots of all guilds as one list, including archived ones,
  and does not know about `archived_at`.

Remove the Django deployment when the Letter web is live (decision 2).

### Kubernetes manifests for `letter-web`

Add the new keys to the existing secret `spot-assistant-web` (it already holds
`DATABASE_*` for the bot and Django):

- `DISCORD_CLIENT_ID`, `DISCORD_CLIENT_SECRET` (Discord developer portal,
  OAuth2 page of the bot application)
- `WEB_BASE_URL` (for example `https://tibialoot.com`). The bot reads
  it too, for its "not premium" reply.
- `WEB_ADMIN_DISCORD_IDS` (the site admins)

Example (not applied; copy the ingress annotations and TLS settings from the
existing `spot-assistant-web` ingress):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: letter-web
  namespace: refugees
  labels: { app: letter-web }
spec:
  replicas: 1
  selector:
    matchLabels: { app: letter-web }
  template:
    metadata:
      labels: { app: letter-web }
    spec:
      containers:
        - name: letter-web
          image: marahin/letter-web:<sha>
          ports:
            - { name: http, containerPort: 8080 }
            - { name: metrics, containerPort: 3005 }
          envFrom:
            - secretRef: { name: spot-assistant-web }
          env:
            - { name: TIBIA_WORLD_API_BASE_URL, value: "http://ext-tibiadata-api:8080/v4" }
            - { name: TZ, value: "Europe/Berlin" }
          startupProbe:
            httpGet: { path: /livez, port: metrics }
            periodSeconds: 10
            failureThreshold: 60
          livenessProbe:
            httpGet: { path: /livez, port: metrics }
          readinessProbe:
            httpGet: { path: /readyz, port: metrics }
          resources:
            requests: { cpu: 50m, memory: 64Mi }
            limits: { memory: 256Mi }
---
apiVersion: v1
kind: Service
metadata:
  name: letter-web
  namespace: refugees
spec:
  selector: { app: letter-web }
  ports:
    - { name: http, port: 80, targetPort: http }
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: letter-web
  namespace: refugees
spec:
  rules:
    - host: tibialoot.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: letter-web
                port: { name: http }
  tls:
    - hosts: [tibialoot.com]
      secretName: letter-web-tls
```

More than one replica is safe: sessions are in the database, and the
experience job runs in one process at a time (advisory lock). The resync
cooldown of Settings is kept per process, so with two replicas a user can
request two refreshes in 5 minutes.

### Discord developer portal

On the bot's application:

1. **OAuth2 > Redirects**: add `<WEB_BASE_URL>/auth/callback`, for example
   `https://tibialoot.com/auth/callback`.
2. The sign-in scopes are `identify guilds guilds.members.read`. They need no
   approval.
3. **Bot > Privileged Gateway Intents**: no change. The bot asks for no
   privileged intent.

### Bot invite

The web's "Add Letter to Discord" button asks for the scopes
`bot applications.commands` and the permissions `268561424`:

| Permission | Why |
|---|---|
| View Channels | Read the command and summary channels. |
| Send Messages | Post the summary and the replies. |
| Manage Messages | Remove the old summary messages. |
| Embed Links | The summary embeds. |
| Attach Files | The summary chart image. |
| Read Message History | Find the previous summary messages. |
| Manage Channels | Create `#letter` and `#letter-summary` when no channel is set. |
| Manage Roles | Create the `Postman` role when no overbook rank is set. |

A server that added the bot before keeps its old permissions. Settings has a
"Re-invite the bot" button that asks for the missing ones.

### Premium

- Celesta Community (`806152499760201738`) has `premium_forever = true` from
  migration `20260926100000`. The web cannot turn it off; only SQL can.
- Every other server starts without premium. A site admin turns it on in
  **Admin > Servers**. The bot applies it at once (`letter_guild_config`).

## Development

- `make generate` compiles the `.templ` files (commit the `*_templ.go` output).
- `make css` builds `internal/infrastructure/web/dist/app.css` with the pinned
  Tailwind CLI (downloaded to `bin/tailwindcss`). The file is not committed.
- Run the web locally with a database and dummy Discord values:
  `DATABASE_HOST=127.0.0.1 TZ=Europe/Berlin WEB_BASE_URL=http://localhost:8080 DISCORD_CLIENT_ID=x DISCORD_CLIENT_SECRET=y make run-web`
  (or `./bin/letter-web` after `make build-only`). `docker compose up -d db` starts
  a database. Sign-in needs real credentials and the local redirect URL in the
  portal.
- `make run-bot` runs the bot; it needs `BOT_TOKEN`.
- `make lint` runs the templ check and golangci-lint. `make help` lists the
  targets.
- The wiring is in `internal/infrastructure/fxmodule` (fx); see its `AGENTS.md`.
