# Letter web panel: decisions for manual review

Each item is a decision taken without the owner. Review and reverse as needed.

## Scope and data

1. **Same repository, new binary.** The web lives in this repository as `cmd/web`. It shares the PostgreSQL database with the bot. The bot and the web are separate processes.
2. **The Django admin is being retired.** `spot-assistant-web` (Django, `spot-assistant-web.tibialoot.com`) is not changed. Its tables stay until a later contract migration, after Django is removed. Until then, the rule "keep the previous release working" (decision 37) covers Django too: it keeps working on the new schema. Decommission it after the Letter web is live.
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
    sends the overbook message on Discord) and `letter_booked` (decision 88).
    Each signal is best-effort. The durable resync flag recovers a missed
    resync. When the listener connects again, the bot refreshes every summary,
    which recovers a missed summary refresh (decision 90).
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
    analytics consent. Web tests use httptest. The go-rod browser e2e suite was
    added later (see "Public stats, premium lock and e2e").

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
    Channels are locked on a server without premium, like every other server
    page (changed later, see "Public stats, premium lock and e2e").
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
    last 20 sessions) stays in the browser's localStorage. The analytics call of
    the original is dropped. Since decision 77 the server stores the session's
    figures (not the pasted text) for the public stats, unless the user unticks
    that. Differences from the original:
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
37. **Both binaries apply the goose migrations on start**, under the Postgres
    advisory lock 7419000, as in scxmanager. They start in any order: the
    second one waits for the lock (at most 10 minutes), then finds nothing
    pending. A database that atlas migrated is adopted once: the atlas versions
    that finished are written to `goose_db_version` and do not run again. A
    partially applied atlas revision, or tables without a migration history,
    stop the start. A migration must keep the previous release working (see
    `AGENT.md`). Exception: `20260926100100` makes three queries of the bot
    before PR #59 fail, so this release deploys the bot first (see `README.md`,
    "Rollout order").
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
50. **Bot forms: buttons, a booking wizard of lists and `/reservations`.** The
    guild summary gets the buttons "Book a respawn", "My reservations" and
    "Open panel" (only when `WEB_BASE_URL` is set). A DM summary has no buttons.
    A guild without upcoming hunts gets a "No upcoming hunts" message with the
    same buttons, so the channel never shows an old summary. Each new booking
    is announced in public (decision 87). A region channel has "Book in
    <region>" instead of "Book a respawn" (decision 101). A member can wait
    for a taken respawn (`/queue`, decisions 105-107). Rules:
    - The member picks from lists (string selects) and does not type. Typed
      names and times caused too many errors. discordgo stays at v0.27.1 (see
      "Parity with scxmanager"), and a Discord form (modal) cannot hold a list,
      so booking is a private message (the wizard) that the bot changes in place.
      "Book a respawn" and "My reservations" in a private message change that
      message; in the summary channel they send a new one.
    - Step 1, the respawn. List 1, "Your usual respawns": the member's most
      booked active respawns of the last 90 days (most bookings first, then the
      latest), filled up to 25 with the most booked respawns of the server of the
      last 30 days ("Popular respawns" when the member has none). No bookings: no
      list. Lists 2 to 4: the active respawns by name, at most 75 on a page, cut
      into up to 3 lists of the same size. Each list is named by the first
      letters of its first and last respawn ("A–F"); when two lists meet on the
      same letters, the bot adds letters ("A–Ka", "Ko–Z", at most 3).
      "‹ Previous" and "More respawns ›" show only with more than 75 respawns.
      "Search by name" opens a form with one text field: an exact name (not
      case-sensitive) or the only name that contains the text goes to step 2;
      more names show one "Matching respawns" list (up to 15); no name shows
      step 1 with the error. Archived respawns never show.
    - Step 2, the time. The start list: "Now" (the current minute when the
      member books), then 24 half-hour starts from the next :00 or :30 (12
      hours). A start inside a reservation of the respawn says "Booked by X
      until 20:00", and the message lists the reservations of these 12 hours,
      so members see the free time. "‹ Earlier" and "Later ›" move by 12 hours,
      up to 48 hours ahead. The length list: 30 min to 3 hours in steps of 30
      min (`booking.MaximumReservationLength`). No default. "Book" is disabled
      until the member picks a start and a length. Times are in the bot time
      zone (Europe/Berlin), and the message names the zone. Starts step in
      elapsed time, so a DST change shows the repeated or the skipped hour.
    - A start that passed when the member presses Book (an old message) starts
      now if it passed less than 30 minutes ago; else the bot shows the error
      and "Try again".
    - A booking that overlaps shows the conflicts. "Overbook them" shows only
      when the overbook would pass (`booking.OverbookAllowed`: the overbook
      right, or one abandoned reservation, never an own one). "Try again"
      returns to step 2 with the choice kept.
    - Create, edit and cancel go through `reservations.Service`, the service of
      the web: the same rights, the same booking rules, the same atomic edit.
      An edit never overbooks. There is no author field; managers choose an
      author in the web.
    - "My reservations" lists only the member's own upcoming reservations, at
      most 4 (Discord allows 5 rows of buttons). Each has "Edit n" and
      "Cancel n"; a cancel asks to confirm. More reservations: use the web
      panel or `/unbook`.
    - "Edit n" changes the list into step 2 for the reservation, with its
      respawn, start and length chosen. A start or a length that is not in the
      list is added first, marked "(current)"; such a start replaces "Now"
      (a list holds 25 options). The button is "Save", and "Back"
      returns to the list. "Change respawn" goes to step 1 and keeps the start
      and the length. An ongoing reservation keeps its start ("Keep (started
      18:00)") and its respawn; only the length changes.
    - Every reply is ephemeral. The public summary refresh (3 s debounce, in
      the bot process through `bot.LocalNotifier`) shows the change to others.
    - Buttons and lists need premium and resolve the member's rights fresh on
      each click. Booking needs the reserve right. The command channel rule of
      `/book` and `/unbook` does not apply, as the replies are ephemeral.
    - The state of a button or a list is in its versioned custom id (`lf1:...`,
      times as Unix seconds, at most 100 characters); the chosen options show
      as selected. The bot stores nothing between clicks. Every id is checked
      again on the server (respawns and reservations of the guild only).
    - The bot answers every click at once with a deferred reply, except the
      "Search by name" click (a form must be the first reply).
    - `/book` keeps its text times (`reservationforms.ParseClock`: H, HH, H:MM,
      HHMM, "." ";" "," or "h" as separator; the current minute is today).
    - The copy is English, in the bot formatter (decision 33). Only the
      overbook button adds a metric (`IncOverbook`). No migration.

## Public stats, premium lock and e2e

The owner asked for Discord links as in scxmanager, a premium lock on every
server page, public stats, `/dev/login` and an e2e suite.

51. **The community invite comes from `DISCORD_INVITE_LINK`**, default
    `https://discord.gg/b7Qq8V2XFR`. Only an `http(s)` value is used; empty
    hides every support link.
52. **The support links are where scxmanager has them:** both footers, the
    sidebar "Get help", the "Help us improve this translation" note, the landing
    page CTA, the 404 and 5xx pages, and the premium lock. A 403 has none. The top
    bar has none.
53. **"Server-related view" means `/servers/{id}` and every route under it**,
    Settings and Channels included. They are all locked on a server without
    premium. This replaces decision 24's exception: one gate (`enterGuild`) for
    every server page, and a test fails on a server route that is not locked.
54. **Not locked:** the landing page, the Loot Calculator, the dashboard, sign-in,
    the language switch and `/admin/*`. The dashboard card of a locked server says
    "Unlock with Premium" and opens the lock page.
55. **Site admins pass the lock**, as before.
56. **The lock comes before the rank check.** Everyone on a locked server sees
    the same page; it does not tell which pages a rank would open.
57. **The lock page uses the exact copy** "Unlock the feature with Premium. Join
    the Discord and get on board!". It names no server and no feature. Its button
    opens the Discord; the second button goes back to the dashboard, or home when
    signed out. The admin's Settings button and the Settings premium notice are
    removed.
58. **Public pages:** the Stats overview, the three tables with the CSV export,
    the respawn and player pages, and the character page. The respawn's
    reservations and all changes still need sign-in and a rank.
59. **Public stats need premium.** A server without premium shows the lock to
    visitors too.
60. **The URLs do not change.** A member and a visitor share each Stats URL.
61. **Visitors see what members see** on the Stats pages; the data was already
    shown to every member. Only a stored server with the bot present is visible.
62. **A failed member lookup gives the public view**, not an error: the page
    needs no sign-in. A session whose user is gone is destroyed, and the visitor
    goes on signed out.
63. **A signed-in visitor who is not a member keeps their own sidebar** and gets
    the server in a "Public stats" bar above the page, with the four Stats tabs.
    A signed-out visitor gets the top bar and a "Log in with Discord" strip that
    comes back to the page. The server does not become the visitor's default.
64. **`/dev/login` needs three things:** the `devauth` build tag,
    `WEB_DEV_AUTH=true` and a `localhost`/`127.0.0.1`/`::1` base URL. Otherwise
    it answers 404. Production images are built without the tag.
65. **A dev login goes through `Auth.Complete`** with a `dev-login:<id>` code. A
    mock OAuth port (devauth build only) stores the mock user and answers its
    servers and roles from the database; real users fall through to Discord.
    The web wires the mock port only under the same conditions as decision 64,
    so `/auth/callback` with a `dev-login:` code fails everywhere else.
66. **The `/dev/login` page is plain English HTML**, as in scxmanager, not a
    templ page with i18n keys.
67. **The seed is an fx app (`seedapp`) behind the devauth tag**, so
    `cmd/seed/main.go` stays one line. Without the tag `cmd/seed` has no files
    to build. The seed is idempotent; it writes reservations only once.
68. **The Docker e2e run uses its own compose project (`letter_bot_e2e`), its
    own volume and other host ports (18080, 13005)**, so it runs beside the dev
    stack and `down -v` removes only its own data. The web and the seed run the
    code built into the image, not a source mount.
69. **CI runs the e2e suite on GitHub Actions** (`scripts/e2e-ci.sh`) with a
    Postgres service and the runner's Chrome, before the image build.
70. **Chrome runs with `--headless=old`**, as in scxmanager: the new headless
    mode hangs on page load in some sandboxes.
71. **The public tab row wraps; it does not scroll.** A scroll container showed a
    stray scrollbar under the underline of the active tab.
72. **The new support links keep scxmanager's `zone-400` text**, as decision 43
    allows for pages other than the Loot Calculator. They open in a new tab,
    with no extra "new tab" text, as in scxmanager.
73. **The character page reads TibiaData only for a character the server knows**:
    one with reservations in the server (in any range) or experience snapshots
    in the selected range. Any other name answers 404 before TibiaData is
    asked, so the public page cannot be used to query TibiaData for any name.
    Follow-up (open): rate limiting of the public pages.
74. **The TibiaData character cache holds at most 1000 entries.** Expired entries
    go first, then the entry that expires soonest.
75. **The CSV export has at most 5000 rows, for everyone.** The HTML tables keep
    500. A member does not need more for a spreadsheet, and one limit is simpler
    than one for visitors and one for members.

## Loot stats, worlds and spawns

The owner's request: no andromeda (the old tibialoot.com telemetry), the split on
the server, loot and experience stats of respawns, players and characters, and
world and spawn stats, visible without sign-in, for several Tibia worlds. The
layout of the four new pages follows the Claude Design mockups in
`docs/web/design/stats/` (`Worlds`, `World`, `Spawns`, `Spawn`; canvas
https://claude.ai/artifact/BZV6jDiWcBeUHoZ384KKin).

76. **No andromeda, no third party.** The Loot Calculator sends nothing anywhere
    but its own form post, and fingerprints nobody. The split stays in
    `lootcalc` on the server; `loot-calculator.js` only copies text and keeps the
    local history.
77. **Loot data comes from Loot Calculator sessions.** Every calculation whose
    text has the `From … to …` line is stored in `loot_sessions` (with one row
    per character in `loot_session_members`), unless the user unticks "Count
    this session in public stats" (ticked by default; a note says what is kept).
    Stored: the From and To times read in the process zone (decision 31), the
    duration, the party totals and each member's name, key (`lower(trim)`),
    leader flag, loot, supplies, balance, damage and healing. The pasted text is
    not stored. A sha256 of the times and the figures (members sorted by key) is
    unique, so a recalculation or a "Load" from the history counts once. A form
    without the box (an older page) counts. Unticking writes nothing and leaves
    earlier sessions as they are.
78. **A session belongs to one reservation, and the author's own session wins.**
    It matches a reservation of a server with premium and the bot present that
    has one of the session's members among its author characters and overlaps
    the session by at least half of the session. Among several: the largest
    overlap, then the latest start, then the larger id. A reservation keeps one
    session (`reservation_loot`). The form is public, so anyone can paste a
    made-up session with a reservation's characters and times; a session posted
    by the signed-in author of the reservation (`loot_sessions.submitted_by`
    equals `web_reservation.author_discord_id`) is marked `by_author` and wins
    over any other, and no one else's session replaces it. Between two sessions
    of the same kind the larger overlap wins; on an equal overlap the session the
    reservation already has stays, so a later paste cannot displace it. The same
    session pasted again keeps `by_author`, and the first submitter stays on a
    stored session. A session that moves to another reservation leaves the first
    one. An edit of a reservation's spot, times or author (on the web or in the
    bot, both through `UpdateReservation`) drops its session in the same
    statement: the match was for the old values, and the next paste matches
    again. The candidate search reads only reservations that start at most
    `booking.MaximumReservationLength` before the session, so
    `web_reservation_guild_start_idx` bounds it.
79. **Sessions that do not count.** No `From … to …` line, a session that ends
    more than 5 minutes in the future (not stored), a session out of bounds (not
    stored: shorter than a minute, longer than `booking.MaximumReservationLength`,
    no named member or more than 10 lines, or a figure beyond ±10^12: a public
    form must not store figures that break the sums), no matching reservation
    (stored, shown nowhere yet), or a reservation that keeps a better session
    (stored). After each calculation a one-line notice says which, and links the
    respawn on the server's public stats when the session counts. A storage
    error shows its own notice; the split still shows.
80. **Loot figures.** Profit is the party balance; profit per hour divides it by
    the session time (not the booked time, as the tile says). Loot and supplies
    show on the respawn, player, character and spawn pages. A respawn, player,
    day, world or spawn counts the whole party; a character counts its equal
    share (party figure / members, rounded down as the split pays it), only for
    sessions it was in. A party member who is not in the author text gets no
    character row. Without a session a figure says "No data" (its own tooltip),
    never 0; a loss keeps its minus sign and is red.
81. **The server stats get loot.** The overview has profit and profit per hour
    tiles and two more leaderboards: respawns by profit per hour (at least 3
    hours of sessions) and characters by profit. The tables and the CSV get
    Profit and Profit/h, sortable (`profit`, `profit_h`). Visibility is as before
    (public on a premium server).
82. **The world and spawn pages are public.** `/worlds`, `/worlds/{world}`,
    `/spawns` (with `?world=`, the sort and the CSV) and `/spawns/{key}` need no
    sign-in and no premium of the visitor. They read only servers with premium,
    the bot present and a world set (`guilds_world`): the data each of those
    servers already shows publicly. There is no world list in the code; a world
    appears when such a server sets it. A world is keyed by `lower(trim(name))`
    and shown with its most common spelling; a spawn is a respawn name keyed the
    same way across servers. An unknown world or spawn answers 404.
83. **The cross-server range has its own cookie** (`letter_range_worlds`), apart
    from each server's. The picker there shows no days with data: that would be
    a query over every server.
84. **"Spawns" in the navigation, "Respawn" in the tables.** The cross-server
    directory is "Spawns" ("Respy"); a server's own list stays "Respawns". Both
    new links sit in the signed-out top bar and in the signed-in sidebar, under
    the Loot Calculator.
85. **The world page's per-hour boards need 3 hours of data**, like the server
    overview, and its character boards link the character page of the server of
    the character's latest reservation.
86. **The spawn page sums its world rows in Go** from the per-server rows (at
    most 500), so it runs one aggregate query, not two.

## Region summaries, queue and booking announcements

The owner's request: every booking is announced to the whole server, as its
record of bookings, and the summary changes only when there is a reason, gently,
because Discord rate-limits the bot.

87. **Each new booking is announced in the command channel** (the configured
    one, else `#letter`), never in a summary channel, because the bot cleans
    and replaces the summary. Bookings from the buttons, the wizard, an
    overbook and the web are announced. Edits and cancels are not. `/book`
    already replies in public, so it is not announced twice. The message
    mentions the author but pings nobody. The bot announces in the
    background. At shutdown it stops the announcements and waits for them.
88. **A web booking reaches the bot through `letter_booked`**, with a JSON
    payload with json tags. `letter_summary_refresh` keeps its guild id payload.
89. **The online dot shows only for a reservation in progress**, in the summary
    and in the DM summary. A dot on every reservation would change the summary
    at each login or logout. So the start of a reservation also changes the
    summary.
90. **No polling of the summary.** It is refreshed on startup (`GuildCreate`),
    after a booking, spot or settings change (in the bot or through NOTIFY), at
    the next start or end of a listed reservation (one timer per server, at most
    24 hours ahead), when the NOTIFY listener connects again, and when someone
    deletes a summary message (decision 98). The 3-second
    debounce per server stays. A failed
    refresh is retried after 30 seconds, then after twice as long each time, up
    to 15 minutes.
91. **The 2-minute tick still reads the online players** (TibiaData, not
    Discord) and asks for a refresh. The bot posts only when the content of the
    summary changed: it keeps a hash of the posted content per channel, in
    memory. A restart posts each summary once.
92. **One summary worker does all summary writes**, one server at a time, with
    at least 1 second between the bot's own writes. It writes through one
    Discord session, so discordgo tracks all rate limits in one place. discordgo
    waits out a 429 by itself.
93. **The summary cleanup deletes messages older than 13 days one by one.**
    Discord refuses those in a bulk delete, and a quiet server can keep a
    summary that long.
94. **A respawn has an optional region from a fixed catalog**
    (`internal/core/regions`), stored as a key in `web_spot.region`. No region,
    or an unknown key, means "Other". Region names are proper nouns and are not
    translated. The migration and the default list import give the default
    respawns their regions (`regions.DefaultSpotRegion`; a test keeps the two
    equal). The default list makes 18 regions by Tibia geography. Places with
    few respawns share a region (e.g. "Darama & Krailos"), and the big quest
    areas make two regions ("POI, Inquisition & Ferumbras", "Endgame Quests").
    32 names that we could not place stay in Other. The owner should review the
    mapping; a manager can change a region in the web.
95. **The summary per region is on by default** (`guilds.region_summaries`).
    An admin switches it on the Settings page, with scxmanager's checkbox,
    Save and status pattern. Saving sends `letter_summary_refresh`: the summary
    pass reads the setting.
96. **Layout.** The category "Letter respawns" holds one text channel per
    region with respawns, `letter-<key>` (e.g. `letter-port-hope`,
    `letter-other`), made in catalog order. The bot never renames, moves or
    edits a channel after it made it, because Discord limits channel edits
    hard. The channels deny Send Messages and Add Reactions to @everyone and
    allow the bot. When Discord refuses the overwrites, the bot makes the
    channel without them. The summary channel (configured or `#letter-summary`)
    becomes an index: a link to each region channel with its number of
    upcoming hunts, and the buttons of the whole server. When the summary
    channel setting changes, the bot deletes the index in the old channel.
97. **The summary per region needs at least one active respawn with a region.**
    Otherwise the server keeps the single summary. When Discord refuses a
    channel (missing permissions, or the limit of 500 channels), the server
    keeps the single summary for an hour, then the bot tries again.
98. **The bot changes Discord gently.** Each region channel stores a hash of
    the content the bot wrote there (`summary_region_channels.content_hash`).
    A pass writes only to the regions whose content changed, also after a
    restart. A failed region does not stop the other regions. A pass makes
    at most one channel create or delete, 5 seconds after the previous write,
    and asks for another pass after 5 seconds, so servers take turns. Message
    writes keep 1 second apart. All writes go through the summary worker's one
    session (decision 92). Before the bot makes a channel, it adopts one of
    the same name in the category, and it asks the API before it decides a
    channel is missing: the gateway cache can lag behind a channel the bot
    just made.
    - The bot keeps the ids of its summary messages in memory. When someone
      deletes one of them (one message or a purge), the bot forgets the hash
      of that channel and refreshes the server. This applies to the single
      summary, the index and the region channels. Deletes in other channels
      cost no database query. The bot's own deletes do not count.
99. **Cleanup.** A region channel that is no longer needed (no active respawns
    in the region, or the setting is off) gets a notice, and the bot deletes
    it 24 hours later. Switched back on within 24 hours, the region gets its
    channel back. The region stays retired until a pass writes its summary,
    so a pass that stops early (e.g. at the channel limit) does not restart
    the 24 hours. The cleanup runs also when the summary or the index write
    fails. The bot deletes the category only when it is empty. A
    category with the server's own channels in it is left to the server. A
    region channel that someone deletes by hand comes back while it is needed
    (scxmanager parity). Without premium, or without the bot, nothing is
    touched.
100. **A region channel shows a header and one embed per booked respawn**,
     packed into messages of at most 10 embeds and about 5800 characters. The
     last message has "Book in <region>", "My reservations" and "Open panel".
     There is no chart; the single summary keeps its chart.
101. **"Book in <region>" limits only step 1 of the wizard** to the respawns
     and the usual respawns of the region, with an "All respawns" button.
     Search and "Change respawn" stay for the whole server. The new custom ids
     (`bookr`, `rrp`) leave the old ones unchanged, so old buttons keep working.
102. **A respawn has an optional image** (`web_spot.image_url`, http or https,
     at most 2048 characters). It shows as the embed thumbnail of the respawn
     in its region channel only, because the single summary uses fields. When
     Discord refuses the message (code 50035), the bot writes the region again
     without images. The images come back when the content changes. The
     web shows a small thumbnail with `referrerpolicy="no-referrer"`.
103. **"Rename" becomes "Edit"** (name, region, image) on the Respawns page,
     at `/servers/{id}/spots/{spot}/edit`. The `/rename` route is removed; it
     had no other users. A closed row loads its form on the first open
     (`GET .../edit`, htmx), so the list does not carry a region picker per
     row. A refused edit renders that row open, with its form.
104. **The summary pass has hooks.** A hook runs at the start of each pass of
     a premium server, before any summary write. Its error is logged and the
     pass goes on. The booking queue sends its due offers there.
105. **The queue.** A member waits for a taken respawn with `/queue` (respawn,
     length) or the "Queue" button of the wizard, which shows when the respawn
     is taken now. `/queue-leave` stops it. The replies are private. The queue
     lives in Discord only (`spot_queue_entries`); the web has no queue page.
     - "Free" means that no reservation of the respawn overlaps [start,
       start + length], edges included, as in booking. The start is the next
       full minute, because booking counts the minute in which the previous
       hunt ended as taken.
     - The length is 30 minutes to 3 hours, in steps of 30 minutes, default
       2 hours. An entry lasts 24 hours. A member waits for at most 3
       respawns per server. Queueing the same respawn again replaces its
       length and expiry.
     - A respawn that is free when the member queues gives the offer at once,
       in the reply.
106. **Offers.** Each summary pass of a premium server deletes the expired
     entries, clears the offer of an entry whose respawn was taken again, and
     offers each free respawn to every member who waits for it, at the same
     time. The first member to book gets it; the others get the normal booking
     error. An entry gets a new offer only after its respawn was taken and
     freed again. The bot sends the offer by DM, through the summary worker's
     session and pacing (decision 92). When Discord refuses the DM (code
     50007), the offer goes to the command channel with a mention that pings
     only that member. An offer that could not be sent is cleared, so a later
     pass tries again.
     - A respawn becomes free only when a reservation ends or is deleted,
       moved or shortened. The end of a listed reservation is a boundary
       timer, and a change triggers a pass (decision 90). The 2-minute tick
       triggers a pass too (decision 91). The queue needs no timer of its own.
       An expired entry counts as gone in every query, so its deletion can
       wait for the next pass.
107. **Book and Cancel of an offer.** The buttons work for 10 minutes and only
     for the member who queued; another member gets a private refusal. A
     click in a DM has no server and no member, so the bot reads the server
     from the entry and the member from the API (with the permissions of
     their roles). Book books from the next full minute for the queued length
     through the normal booking, so the booking is announced (decision 87) and
     the summary changes. Any new booking of the respawn by the member removes
     their entry. Cancel removes the entry. A newer offer replaces an older
     one. When the respawn was taken meanwhile, Book shows the booking error
     and keeps Cancel.

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
- **goose migrations at start** (decision 37), embedded in both binaries, with
  sqlc reading the migrations directory (`schema.sql` is removed). Differences:
  the goose `Provider` API instead of the package globals, and a one-time
  adoption of the atlas history.
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
- sqlc keeps one config for each repository package; mockery stays at v3.8.0.
- GitHub Actions (scxmanager: GitLab CI); the Dockerfile targets stay.
- Health paths stay `/livez` and `/readyz` (scxmanager: `/healthz`).
- Only Tailwind `--minify` (decision 18); no `make minify` or `lint-dupes`.
- discordgo stays at v0.27.1 (scxmanager: v0.29.0). The shards library and
  the bot's gateway code depend on it, and no test covers a live gateway.

Follow-ups: the rename to `internal/infra`. (The go-rod e2e suite is done.)

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
8. **Rate limiting of the public writes.** Anyone can post Loot Calculator
   sessions, and a counted one changes the public stats (decisions 73, 77, 78).
   There is no rate limit yet. Should there be one?
9. **The analyser's time zone.** The analyser prints the player's local time
   without a zone, and the server reads it in `Europe/Berlin` (decision 31). A
   player in another zone gets no match, or a wrong one. Should the calculator
   send the browser's zone, or ask for it?
10. **Unmatched sessions.** They are stored and shown nowhere. A follow-up could
    attribute them to a world through TibiaData; until then, should they be
    deleted after some time?
