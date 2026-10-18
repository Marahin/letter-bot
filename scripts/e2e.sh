#!/usr/bin/env sh
# The whole e2e cycle in Docker: start the e2e stack (docker-compose.e2e.yml) from
# an empty database, wait for the web, run the browser suite, remove the stack.
# It touches only its own compose project, so a running dev stack stays as it is.
# `make e2e` runs the suite alone, against a stack that is already up.
set -e

PROJECT="${E2E_PROJECT:-letter_bot_e2e}"
COMPOSE="docker compose -p $PROJECT -f docker-compose.yml -f docker-compose.e2e.yml"
E2E_WEB_PORT="${E2E_WEB_PORT:-18080}"
E2E_METRICS_PORT="${E2E_METRICS_PORT:-13005}"
export E2E_WEB_PORT E2E_METRICS_PORT
# docker-compose.yml reads these for the db; docker-compose.e2e.yml pins the values.
export DATABASE_USER="${DATABASE_USER:-postgres}" DATABASE_PASSWORD="${DATABASE_PASSWORD:-postgres}" DATABASE_NAME="${DATABASE_NAME:-postgres}"
export E2E_BASE_URL="${E2E_BASE_URL:-http://localhost:$E2E_WEB_PORT}"
export E2E_HEALTH_URL="${E2E_HEALTH_URL:-http://localhost:$E2E_METRICS_PORT/readyz}"
# The image compiles the web and runs the seed on start.
TIMEOUT_SECONDS=300

echo "==> removing an earlier e2e stack and its database"
$COMPOSE down -v --remove-orphans >/dev/null 2>&1 || true

echo "==> starting the e2e stack (db, seed, web)"
$COMPOSE up -d --build db seed web

echo "==> waiting for $E2E_HEALTH_URL (up to ${TIMEOUT_SECONDS}s)"
elapsed=0
until curl -fsS "$E2E_HEALTH_URL" >/dev/null 2>&1; do
	if [ "$elapsed" -ge "$TIMEOUT_SECONDS" ]; then
		echo "==> the web did not become ready within ${TIMEOUT_SECONDS}s" >&2
		$COMPOSE logs seed web >&2 || true
		$COMPOSE down -v
		exit 1
	fi
	sleep 2
	elapsed=$((elapsed + 2))
done
echo "==> ready after ${elapsed}s"

echo "==> running the e2e suite"
set +e
make e2e
TEST_EXIT=$?
set -e

echo "==> removing the e2e stack"
$COMPOSE down -v

exit $TEST_EXIT
