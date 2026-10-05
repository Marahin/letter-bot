APP ?= spot-assistant-bot
WEB_APP ?= letter-web
TAG ?=
# An empty TAG (e.g. Docker's unset build-arg) falls back too, not just an undefined one.
ifeq ($(strip $(TAG)),)
TAG := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
endif
REGISTRY ?= registry.marahin.pl
BOT_BIN ?= spot-assistant-bot
WEB_BIN ?= letter-web
TEMPL_VERSION ?= v0.3.1020
TAILWIND_VERSION ?= v3.4.17
TAILWIND ?= ./bin/tailwindcss
# golangci-lint is pinned: it is the blocking quality gate, so an upgrade must not
# turn CI red on unrelated changes. `go run` builds it once into the module cache.
GOLANGCI_VERSION ?= v2.13.2
GOOSE_VERSION ?= v3.27.1
MIGRATIONS_DIR := internal/infrastructure/db/postgresql/migrations
MIGRATIONS_BASE ?= origin/main
CSS_OUT := internal/infrastructure/web/dist/app.css
LDFLAGS := -X spot-assistant/internal/common/version.Version=${TAG}

.DEFAULT_GOAL := help

.PHONY: help
help: ## List the targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

install-bins: ## Install pinned dev tools (sqlc, mockery, templ)
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.26.0
	@go install github.com/vektra/mockery/v3@v3.8.0
	@go install github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION)

go-mod: ## Run go mod tidy
	@echo "INFO: Running go mod tidy"
	@go mod tidy

install-dependencies: install-bins go-mod ## Install the dev tools and tidy the modules
	@echo "INFO: Downloading dependencies"

$(TAILWIND):
	@mkdir -p $(dir $(TAILWIND))
	@curl -fsSL -o $(TAILWIND) https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-linux-x64
	@chmod +x $(TAILWIND)

css: $(TAILWIND) ## Compile Tailwind CSS into the embedded asset
	@echo "INFO: Compiling Tailwind CSS"
	@# Unlink first: a root-owned app.css left by the compose dev container can't be opened in place.
	@rm -f $(CSS_OUT)
	@$(TAILWIND) -c assets/tailwind/tailwind.config.js -i assets/tailwind/styles.css -o $(CSS_OUT) --minify

generate: ## Generate templ components
	@echo "INFO: Generating templ components"
	@templ generate
	@find . -name '*_templ.go' -not -path './.git/*' -exec gofmt -w {} +

templ-diff: ## Verify templ generated code is up to date
	@echo "INFO: Checking templ generated code is up to date"
	@# Locally, compare content before/after generating, so uncommitted work passes.
	@# In CI (CI set), any untracked or modified *_templ.go also fails: a new one
	@# was never committed.
	@before=$$(find . -name '*_templ.go' -not -path './.git/*' -exec md5sum {} + | sort); \
	$(MAKE) -s generate; \
	after=$$(find . -name '*_templ.go' -not -path './.git/*' -exec md5sum {} + | sort); \
	if [ "$$before" != "$$after" ]; then \
		echo "ERROR: templ generated code is out of date, run make generate"; \
		exit 1; \
	fi; \
	if [ -n "$$CI" ] && [ -n "$$(git status --porcelain -- '*_templ.go')" ]; then \
		echo "ERROR: templ generated code is not committed:"; \
		git status --porcelain -- '*_templ.go'; \
		exit 1; \
	fi

# Renders the PNG icons from dist/favicon.svg (needs rsvg-convert). 16px gets a
# thicker stroke so the bag still reads; the apple-touch icon is full-bleed.
FAVICON_DIR := internal/infrastructure/web/dist
favicons: ## Render the PNG favicons from dist/favicon.svg (needs rsvg-convert)
	@echo "INFO: Rendering favicons"
	@cd $(FAVICON_DIR) && for s in 32 48 192 512; do rsvg-convert -w $$s -h $$s favicon.svg -o favicon-$$s.png; done
	@cd $(FAVICON_DIR) && sed 's/stroke-width="8"/stroke-width="12"/' favicon.svg | rsvg-convert -w 16 -h 16 -o favicon-16.png
	@cd $(FAVICON_DIR) && sed 's/rx="24"/rx="0"/' favicon.svg | rsvg-convert -w 180 -h 180 -o apple-touch-icon.png

mocks: ## Regenerate mockery mocks
	@echo "INFO: Generating mocks"
	@mockery

docker: ## Build the bot Docker image
	@echo "INFO: Building bot Docker image"
	@docker build --target bot --build-arg TAG="${TAG}" -t "${REGISTRY}/${APP}:${TAG}" -f Dockerfile .

docker-web: ## Build the web Docker image
	@echo "INFO: Building web Docker image"
	@docker build --target web --build-arg TAG="${TAG}" -t "${REGISTRY}/${WEB_APP}:${TAG}" -f Dockerfile .

push-to-registry: ## Push the bot Docker image
	@docker push "${REGISTRY}/${APP}:${TAG}"
	@echo "INFO: Pushed ${REGISTRY}/${APP}:${TAG}"

push-web-to-registry: ## Push the web Docker image
	@docker push "${REGISTRY}/${WEB_APP}:${TAG}"
	@echo "INFO: Pushed ${REGISTRY}/${WEB_APP}:${TAG}"

sqlc-diff: ## Verify sqlc generated code is up to date
	@echo "INFO: Running sqlc diff"
	@sqlc diff -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/worldname/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/guild/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/webuser/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/experience/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/stats/postgresql/sqlc.yaml

migration: ## Create a goose migration: make migration name=add_x
	@test -n "$(name)" || { echo "ERROR: set name, e.g. make migration name=add_x"; exit 1; }
	@go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir $(MIGRATIONS_DIR) create $(name) sql

# goose refuses an out-of-order migration only at runtime, on a database that already
# has a newer one; this catches it before the merge.
migrations-order: ## Fail if a migration added since MIGRATIONS_BASE (default origin/main) is not newer than every base migration
	@echo "INFO: Checking the migration order against $(MIGRATIONS_BASE)"
	@git rev-parse --verify -q "$(MIGRATIONS_BASE)^{commit}" >/dev/null || { echo "ERROR: unknown ref $(MIGRATIONS_BASE), fetch it or set MIGRATIONS_BASE"; exit 1; }; \
	base_max=$$(git ls-tree --name-only "$(MIGRATIONS_BASE)" -- $(MIGRATIONS_DIR)/ | xargs -r -n1 basename | grep -E '^[0-9]+_.*\.sql$$' | cut -d_ -f1 | sort -n | tail -1); \
	if [ -z "$$base_max" ]; then echo "INFO: $(MIGRATIONS_BASE) has no migrations, skipping"; exit 0; fi; \
	fork=$$(git merge-base "$(MIGRATIONS_BASE)" HEAD); \
	bad=0; \
	for f in $$(git diff --name-only --diff-filter=A "$$fork" HEAD -- $(MIGRATIONS_DIR)/ | xargs -r -n1 basename | grep -E '^[0-9]+_.*\.sql$$'); do \
		v=$${f%%_*}; \
		if [ "$$v" -le "$$base_max" ]; then echo "ERROR: $$f is not newer than $$base_max on $(MIGRATIONS_BASE); rename it to a newer version"; bad=1; fi; \
	done; \
	exit $$bad

test-db: ## Run the database tests (needs LETTER_TEST_DATABASE_URL)
	@test -n "$$LETTER_TEST_DATABASE_URL" || { echo "ERROR: set LETTER_TEST_DATABASE_URL, e.g. postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable"; exit 1; }
	@echo "INFO: Running the database tests"
	@go test -race -count=1 \
		./internal/infrastructure/db/postgresql/... \
		./internal/infrastructure/experience/... \
		./internal/infrastructure/stats/...

test: install-dependencies sqlc-diff lint css ## Run lint, sqlc diff and the test suite
	@echo "INFO: Running tests"
	@go test -cover -race -coverprofile=coverage.out ./...

test-coverage: test ## Open the HTML coverage report
	@echo "INFO: Generating test coverage report"
	@go tool cover -html=coverage.out

# Run go vet
go-vet: ## Run go vet
	@echo "INFO: Running go vet"
	@go vet ./...

lint: install-dependencies css ## Run golangci-lint (pinned, see .golangci.yml) and the templ check
	@echo "INFO: Running lint"
	@$(MAKE) -s templ-diff
	@go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...

sqlc-generate: ## Generate sqlc code
	@echo "INFO: Generating sqlc"
	@sqlc generate -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/worldname/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/guild/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/webuser/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/experience/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/stats/postgresql/sqlc.yaml

sqlc-vet: ## Run sqlc vet
	@echo "INFO: Running sqlc vet"
	@sqlc vet -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/worldname/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/guild/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/webuser/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/experience/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/stats/postgresql/sqlc.yaml

build: install-dependencies sqlc-generate test ## Generate, test and build both binaries
	@make build-only

.PHONY: run-web
run-web: generate css ## Run the web panel locally (reads the environment, see docs/web/README.md)
	@go run -ldflags="${LDFLAGS}" ./cmd/web

.PHONY: run-bot
run-bot: ## Run the Discord bot locally (reads the environment)
	@go run -ldflags="${LDFLAGS}" ./cmd/bot

build-only: ## Build both binaries without tests
	@echo "INFO: Building version: ${TAG}"
	@CGO_ENABLED=0 go build -o ./bin/${BOT_BIN} -ldflags="${LDFLAGS}" ./cmd/bot
	@CGO_ENABLED=0 go build -o ./bin/${WEB_BIN} -ldflags="${LDFLAGS}" ./cmd/web

bench: install-dependencies go-vet ## Run the benchmarks
	@echo "INFO: Running benchmarks"
	@go test -bench='.' -benchmem ./...> _bench.out
	benchstat _bench.out
	@rm _bench.out

bench-long: install-dependencies go-vet ## Run the long benchmarks
	@echo "INFO: Running long benchmarks. Go make a coffee!"
	@go test -bench='.' -count=6 -timeout=5m -benchmem ./...> _bench.out
	benchstat _bench.out
	@rm _bench.out