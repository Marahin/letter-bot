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

## Ambiguities in the work item

These items of the request were unclear or had a cost. Each has a decision.

1. **"Use Claude Design".** Claude Design was used for the one page without a
   scxmanager equivalent that needed a new layout: the Loot Calculator. The
   mockup is the Claude Design canvas
   https://claude.ai/artifact/GAWJxJW1i9aV72DnEYCQ3n. Its sources are in
   `docs/web/design/` (`Main.dc.html`: paste form with an inline error;
   `Result.dc.html`: split result, transfers with copy buttons, players table,
   history). The `ui-ux-pro-max` skill gave the form and feedback rules (inline
   error under the field, announced copy feedback, 44 px touch targets). The
   scxmanager tokens stay 1:1: the `zone` ramp, the `signal` orange, Space
   Grotesk / Hanken Grotesk / JetBrains Mono.
2. **"The bot should be inactive" in a non-premium server.** A fully inactive bot
   would make Settings useless (no channel or role sync). In a non-premium server
   the bot still records the server, copies its channels and roles and registers
   its commands. It answers each command with a short ephemeral "not premium"
   reply that names the web address, posts no summary and creates no channels or
   roles (decision 6).
3. **Experience stats are sparse.** They cover only characters in the world top
   1000 (decision 10). Every experience figure says "No data", never 0.
4. **The Django admin still writes to `web_spot`.** A spot it creates has no
   `guild_id` and the bot and the web do not see it. This is acceptable because
   the Django admin is superseded (decision 2). See `README.md`, "The Django admin".
5. **"1:1" with scxmanager.** The visual design and the web conventions (templ +
   htmx + Tailwind, `Deps`, `Router`, the middleware chain, CSRF, i18n catalogs,
   `web.Chart`, `web.RangePicker`, `web.Combobox`) are kept. The parts that exist
   for SEO or for scale that Letter does not have are dropped (decisions 17-20).

## Stack and structure

12. **fx wires both binaries.** The owner asked for parity with scxmanager.
    `cmd/web` and `cmd/bot` are `fx.New(<bin>app.App()).Run()`; the wiring is in
    `internal/infrastructure/fxmodule` (see "Parity with scxmanager").
13. **The bot binary moves to `cmd/bot`.** The web is `cmd/web`. Both images
    build from one Dockerfile with two targets. `bot` stays the default target,
    so the current deployment (`/spot-assistant-bot`) is unchanged. CI pushes
    `marahin/letter-bot` and `marahin/letter-web`.
14. **Rank membership comes from the user's OAuth token, not from bot member
    sync.** The web asks for `identify guilds guilds.members.read` and caches the
    user's server list and member record for 5 minutes (a stale entry is used
    when Discord fails). The bot requests no privileged intent: if the portal
    toggle is off, a privileged intent stops the gateway.
15. **Channels and roles reach the web through the bot.** The bot copies them
    into `guild_channels`/`guild_roles` on join, on Discord channel and role
    events (5 s debounce per server), on each tick when a resync is requested,
    and on the "Refresh server data" button.
16. **The web signals the bot with Postgres NOTIFY:** `letter_summary_refresh`,
    `letter_guild_resync`, `letter_guild_config`, `letter_overbooked` (the bot
    sends the overbook message on Discord). Each signal is best-effort. The
    2-minute tick and the durable resync flag recover a miss.
17. **No locale-prefixed URLs.** Language resolution is `?lang=` > `letter_lang`
    cookie > Accept-Language > English. There is no sitemap or hreflang, because
    the panel sits behind sign-in.
18. **No esbuild minification.** Tailwind `--minify` covers the CSS. The JS is
    small.
19. **`*_templ.go` files are committed.** `make test` fails when `make generate`
    changes them; in CI it also fails on an uncommitted one. `dist/app.css` is
    generated by `make css` in every build path and is not committed. Tailwind
    CLI v3.4.17 is pinned.
20. **Dropped scx features:** time-zone dialog, impersonation, setup wizard,
    analytics consent, go-rod browser e2e. Web tests use httptest.

## Permissions, premium and Discord behaviour

21. **The command channel is enforced only when it is set.** An empty command
    channel keeps today's behaviour: `/book` and `/unbook` work in every channel.
    The Channels page therefore labels the empty choice "Any channel (default)",
    not "#letter". An empty summary channel falls back to `#letter-summary`.
22. **The bot creates the legacy channels and the `Postman` role only for a
    premium server, and only when the matching setting is empty** (and, for a
    channel, when no channel has that name).
23. **Permission rules.** They live in one core function
    (`internal/core/permission`) used by the bot and the web:
    - Admin = owner or Administrator.
    - Manage = admin or a manage rank.
    - Reserve = manage, or a reserve rank; no reserve rank set = everyone.
    - View = manage, a view rank, or reserve.
    - Overbook = manage, or an overbook rank; with no overbook rank set, the
      `Postman` role counts.

    Managers and admins can now overbook on Discord without the Postman role.
    Settings has no "Everyone" box: each rank list says what "none checked" means.
24. **Channels and Settings are for the owner and administrators only, as in
    scxmanager.** Managers manage respawns and every reservation. Settings and
    Channels work on a non-premium server; an admin who opens such a server lands
    on Settings. Every other server page shows "Premium required".
25. **Site admins (`WEB_ADMIN_DISCORD_IDS`) have full access to every stored
    server.** They turn `premium` on and off. `premium_forever` is read-only in
    the web; only the migration sets it. Their server switcher lists the servers
    the bot has left too, after the others. Other users see a server only while
    the bot is in it.
26. **The experience job stores snapshots only for tracked characters, and only
    when the value changed.** Tracked means the character is in the author of a
    reservation that is upcoming, active or ended in the last 24 hours, on a
    premium server. A run that sees an unchanged value moves `last_seen_at` of
    the latest snapshot, so a snapshot says "this value was seen from
    `observed_at` to `last_seen_at`". Each complete read is a run, with
    `observed_at` = the TibiaData scrape time (`information.timestamp`, or the
    request time) minus `highscore_age` minutes, the oldest of all pages. An
    empty page, or a page before the last one with fewer than 50 rows, fails
    the run: a partial answer would make tracked characters look absent.
    - Start value: the latest snapshot at or before `start_at`, if it was seen at
      most 2 hours before `start_at`; else the first snapshot within 20 minutes
      after `start_at`.
    - End value: the latest snapshot at or before the first run observed at or
      after `end_at`, if it was seen at or after `end_at`. If that run did not
      see the character, the next run stands in when it was observed within two
      job intervals (one missed run is often a TibiaData hiccup); the
      reservation waits until that next run exists.
    - A missing start or end gives `no_data`.

    The `last_seen_at` checks replace the plain "last snapshot at or before"
    rule of the plan: without them a character that was untracked or outside the
    top 1000 for days got a gain that covered those days. This keeps decision 9
    under deduplication.
27. **The experience job runs in the web process** at start and then every 15
    minutes, under a Postgres advisory lock (`7419001`), so only one web process
    runs it. It does not call TibiaData for a world without tracked characters.
    A gain is stored raw, and may be negative after a death.
28. **Respawn names are unique per server among active respawns, not
    case-sensitive.** The migration archives the second "empty" spot.
29. **Branding: the name "Letter" with an envelope favicon (SVG + PNG).** The
    scxmanager palette and fonts are reused unchanged. There is no `.ico`. The
    CSS class, cookie and JS-global prefixes `scx`/`SCX` become
    `letter`/`LETTER`.

## Web behaviour and scope

30. **UI vocabulary.** English "Respawn", Polish "Resp", matching the bot's
    `respawn` option. Code keeps "spot". See `VOCABULARY.md`.
31. **The web uses the process time zone (`TZ=Europe/Berlin`), the same as the
    bot.** It applies to input, display and daily stat buckets. The web does not
    start without `TZ`: PostgreSQL buckets the stat days in that zone by name.
32. **The Loot Calculator is public and computes on the server.** History (the
    last 20 sessions) stays in the browser's localStorage, never on the server.
    The analytics call of the original is dropped. Differences from the original:
    - The parser is line based and strict: each player block needs Loot,
      Supplies, Balance, Damage and Healing, or the page names the player and
      the missing lines. The original skipped such a block without a word. The
      `From … to …` header is optional (the original failed without it).
    - The split is a faithful port, including floor division for a negative
      total.
    - The TeamSpeak text prints "gp" once, not twice. The clipboard texts stay
      English, like the bot (decision 33).
    - Amounts use dot grouping and a true minus sign on every language.
33. **The bot copy stays English.** There is no `guilds.language` column and no
    bot translation in this branch. The web is PL/EN. The Polish catalog is
    written by hand but marked "pending native review".
34. **Web reservation rules:**
    - Create and edit use the bot's rules: the 3-hour maximum and 3 hours per 24
      hours per author. The start of a new reservation cannot be in the past.
    - Overbooking is possible only on create. Overlap is inclusive, as in the
      bot: 10:00-11:00 and 11:00-12:00 conflict.
    - Edit is refused on a conflict and on an ended reservation. An ongoing
      reservation keeps its start and its respawn.
    - Owners edit and delete their own reservations until they end.
    - Managers edit and delete any reservation, and may delete past ones. Only
      managers change the author.
    - A member books as their server nick. A manager may book for a known author
      (picked from past reservations, 3 or more letters typed) or for a
      free-text author. A free-text author has no Discord id
      (`author_discord_id = ''`), no quota and no owner, and gets no overbook
      message. A posted Discord id must be a snowflake (digits only, at most 20).
35. **The Tibia world moves to Server Settings.** `/world-set` stays.
36. **Stats hours are booked hours** (reservation length), not measured play
    time.
    - The range is the span from the first to the last picked day (at most 400
      days, default 30). The range cookie keeps only its first and last day.
    - Experience per hour divides by the hours of reservations that have
      experience data. A respawn or player row counts every party member's gain;
      a character row only its own.
    - Players are Discord users; free-text authors are left out of the player
      table only.
    - Tables sort and export CSV on the server, and show at most 500 rows.
    - The daily charts mark today as not over yet (dashed last segment).
37. **Migrations stay manual with atlas (`bin/migrate`).** Apply them and roll
    out the new bot image in one step: migration `20260926100100` makes three
    queries of the old bot fail (see `README.md`, "Rollout order"). Order:
    migrate, bot at once, web.
38. **Web sessions live in the `web_sessions` table (scs pgxstore) with a 7-day
    lifetime.** Users and their OAuth tokens live in `web_users`. The prefixes
    keep them apart from the Django tables.

## Added during implementation

39. **`DISCORD_CLIENT_ID` and `DISCORD_CLIENT_SECRET` are required.** The web
    does not start without them, because no one could sign in.
40. **The "Refresh server data" cooldown (5 minutes per server) is kept in the
    memory of each web process**, as in scxmanager. A restart or a second
    replica resets it; the bot's resync is cheap.
41. **The default respawn list has 208 names** (the names in `seeds/spots.sql`;
    the plan said 229, which was its line count).
42. **Removing a respawn is one conditional delete.** The delete runs only when
    no reservation (of any guild id) points at the respawn; otherwise the
    respawn is archived. This has no race and also covers the five 2023
    reservations of another guild id that point at Celesta spots.
43. **The Loot Calculator uses `text-zone-300` for help text** (contrast 4.5:1
    or more on its cards). The other pages keep the scxmanager `zone-400`
    tertiary text 1:1.
44. **mockery is pinned to v3.8.0** (v3.5.4 fails under Go 1.27). Interface
    names must be unique in the module, because every mock goes to one directory.
45. **Sign-out clears the stored Discord token.** The user's other sessions must
    then sign in again once their cached guild data (5 minutes) expires.
46. **A failed htmx request shows a toast** (`htmx-errors.js`): the plain-text
    error body the handler sent, or a generic localized message, in the
    scxmanager toast style. htmx itself swaps nothing on a 4xx or 5xx.
47. **The site is TibiaLoot.com, the bot is Letter.** The web panel is branded
    TibiaLoot.com (`branding.Name`) on `https://tibialoot.com`, and every footer
    says "Not affiliated with CipSoft." The bot, the Go packages, the
    `letter-web` binary and image, the `letter-` CSS prefix and the storage keys
    keep the Letter name.
48. **The landing page opens with the Loot Calculator**, the most used feature.
    The bot's features and invite follow below it.
49. **The selected server is stored per user** (`web_users.default_guild_id`), as
    in scxmanager, not in the session: it survives sign-out and a new browser.
    Without one, the first premium server is selected, else the first server.

## Parity with scxmanager

The owner asked for the same stack and code style as scxmanager (review of
PR #59). This branch adopts:

- **uber-go/fx** for both binaries, with the scxmanager layout:
  `internal/infrastructure/fxmodule` (shared modules) and `botapp`/`webapp`.
  fx handles SIGINT and SIGTERM and stops the hooks in reverse order. A busy
  listen port or an unreachable database fails the start with exit code 1.
- **golangci-lint v2** with scxmanager's `.golangci.yml`, and the uber-go style
  guide vendored as `style.md`. It replaces the separate gofmt, go vet, gocyclo
  and staticcheck steps.
- **A Prometheus registry for each binary**, with the Go and process collectors.
  Nothing registers on the global default registry.
- **`make help`, `make run-web`, `make run-bot`**; Go 1.27 in `.go-version` and
  `shell.nix`; pgx v5.10, zap v1.27.1, x/sync and x/text as in scxmanager.
- **Config loaders return errors** instead of panicking (`postgresql.LoadConfig`,
  `worldapi.LoadConfig`, `bot.LoadConfig`). `DATABASE_SSL` is now used
  (`sslmode`, default `disable`). The bot adapter has no `init()` any more.
- **`go.uber.org/automaxprocs` is removed.** Go 1.25 and later read the container
  CPU limit.

Exclusions in `.golangci.yml` that scxmanager does not have:

- `staticcheck` QF1008 ("could remove embedded field from selector"):
  `r.Spot.Name` on a `ReservationWithSpot` is clearer than `r.Name`. It is a
  quick-fix suggestion, like the QF1003 that scxmanager already excludes.
- `misspell` ignores `Victoris`, a Tibia world.
- `forbidigo` on `web/assets.go` and `i18n/i18n.go`: the same fail-fast panics
  over embedded files that scxmanager excludes for its asset and i18n packages.
- Two `//nolint` lines with a reason: `nilnil` in `experience.optional` (a
  missing snapshot is nil, not an error) and `musttag` on the overbooked NOTIFY
  payload (its field names are the wire format between bot and web versions).

Kept differences, on purpose:

- `internal/infrastructure` keeps its name (scxmanager: `internal/infra`) and
  the ports stay in `internal/ports`.
- atlas migrations applied by hand (scxmanager: goose at start), because the
  Django admin shares the tables. The binaries do not migrate.
- sqlc keeps one config for each repository package; mockery stays at v3.8.0.
- GitHub Actions (scxmanager: GitLab CI); the Dockerfile targets stay.
- Health paths stay `/livez` and `/readyz` (scxmanager: `/healthz`).
- Only Tailwind `--minify` (decision 18); no `make minify` or `lint-dupes`.
- discordgo stays at v0.27.1 (scxmanager: v0.29.0). The shards library and
  the bot's gateway code depend on it, and no test covers a live gateway.

Follow-ups: a go-rod browser e2e suite (decision 20) and the rename to
`internal/infra`.

## Open questions for the owner

1. **Site admins.** Which Discord user ids go into `WEB_ADMIN_DISCORD_IDS`?
2. **Celesta ranks.** After the rollout, only the owner and the administrators
   are managers, and every member can view and reserve (as today). Which roles
   should be the manage, view, reserve and overbook ranks? Until an overbook rank
   is set, `Postman` keeps working.
3. **Production table constraints.** The migrations assume that `web_spot` has
   no unique index on `name` alone. The Django dump may have one (and may have
   an EXCLUDE constraint on `web_reservation` that a fresh database lacks).
   Check `\d web_spot` and `\d web_reservation` on production before migrating:
   a unique index on `name` blocks the same respawn name in two servers.
4. **Rollout window.** Accept the short break of `/unbook` autocomplete and the
   summaries between the migration and the new bot pod (about one minute), or
   choose a quiet hour.
5. **Overbook by managers on Celesta.** Owners, administrators and managers can
   now overbook without `Postman` (decision 23). Confirm this is wanted.
6. **Polish copy.** The Polish catalog needs a review by a native speaker.
7. **Django admin.** When may `spot-assistant-web` (Django) be removed?
