#!/usr/bin/env sh
# The e2e cycle without Docker, for CI: the job provides Postgres (DATABASE_*) and
# Chrome (E2E_CHROME_BIN). It builds the devauth web, seeds, starts the web in the
# background and runs the browser suite against it. Run `make generate css` first.
set -e

E2E_WEB_PORT="${E2E_WEB_PORT:-18080}"
E2E_METRICS_PORT="${E2E_METRICS_PORT:-13005}"
export WEB_ADDR=":$E2E_WEB_PORT"
export WEB_METRICS_ADDR=":$E2E_METRICS_PORT"
export WEB_BASE_URL="http://localhost:$E2E_WEB_PORT"
export E2E_BASE_URL="$WEB_BASE_URL"
export E2E_HEALTH_URL="http://localhost:$E2E_METRICS_PORT/readyz"
TIMEOUT_SECONDS=120

echo "==> building the devauth web"
make build-only WEB_BUILD_TAGS=devauth

echo "==> seeding the e2e servers"
make seed

echo "==> starting the web"
./bin/letter-web &
WEB_PID=$!
trap 'kill "$WEB_PID" 2>/dev/null || true' EXIT

echo "==> waiting for $E2E_HEALTH_URL (up to ${TIMEOUT_SECONDS}s)"
elapsed=0
until curl -fsS "$E2E_HEALTH_URL" >/dev/null 2>&1; do
	if ! kill -0 "$WEB_PID" 2>/dev/null; then
		echo "==> the web exited before it was ready" >&2
		exit 1
	fi
	if [ "$elapsed" -ge "$TIMEOUT_SECONDS" ]; then
		echo "==> the web did not become ready within ${TIMEOUT_SECONDS}s" >&2
		exit 1
	fi
	sleep 2
	elapsed=$((elapsed + 2))
done
echo "==> ready after ${elapsed}s"

echo "==> running the e2e suite"
make e2e
