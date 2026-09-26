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
