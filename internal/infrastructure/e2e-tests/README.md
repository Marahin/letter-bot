# Browser e2e suite

The suite drives a running devauth stack with a headless system Chrome
([go-rod](https://github.com/go-rod/rod)): sign-in through `/dev/login`, the
premium lock, the public stats and the support links. Every file has
`//go:build e2e`, so `make test` does not run it.

## Run

- `make e2e-stack` (`scripts/e2e.sh`) does the whole cycle in Docker: it starts
  `db`, `seed` and `web` from `docker-compose.yml` and `docker-compose.e2e.yml`
  under the compose project `letter_bot_e2e`, waits for `/readyz`, runs the suite
  and removes the stack with its volume. The stack publishes ports 18080 (web)
  and 13005 (metrics), so the dev stack can run at the same time.
- `make e2e` runs the suite alone, against a stack that is already up.
- CI (`.github/workflows/ci.yml`, job `e2e`) runs `scripts/e2e-ci.sh` without
  Docker: Postgres is a job service, and the script builds the devauth web, seeds
  and starts it.

The seed (`cmd/seed`, `internal/infrastructure/devauth`) writes two servers:
`700000000000000900` (premium, "Letter E2E", with respawns and 30 days of
reservations) and `700000000000000901` (no premium). The mock users are listed on
`/dev/login`.

## Environment

- `E2E_BASE_URL`: default `http://localhost:18080`.
- `E2E_HEALTH_URL`: default `http://localhost:13005/readyz`.
- `E2E_GUILD_ID`, `E2E_LOCKED_GUILD_ID`: the seeded servers.
- `E2E_SUPPORT_URL`: the `DISCORD_INVITE_LINK` of the stack, default `https://discord.gg/b7Qq8V2XFR`.
- `E2E_CHROME_BIN`: default `/usr/sbin/google-chrome-stable`.

Chrome's new headless mode hangs on page load in some sandboxes, so the suite
starts Chrome with `--headless=old`. The suite blocks `discord.com`: `/login`
redirects there, and the tests assert on the blocked URL.
