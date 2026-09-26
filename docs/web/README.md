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
