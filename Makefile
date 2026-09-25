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
CSS_OUT := internal/infrastructure/web/dist/app.css
LDFLAGS := -X spot-assistant/internal/common/version.Version=${TAG}

install-bins:
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.26.0
	@go install github.com/vektra/mockery/v3@v3.8.0
	@go install github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION)

go-mod:
	@echo "INFO: Running go mod tidy"
	@go mod tidy

install-dependencies: install-bins go-mod
	@echo "INFO: Downloading dependencies"

$(TAILWIND):
	@mkdir -p $(dir $(TAILWIND))
	@curl -fsSL -o $(TAILWIND) https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-linux-x64
	@chmod +x $(TAILWIND)

css: $(TAILWIND)
	@echo "INFO: Compiling Tailwind CSS"
	@# Unlink first: a root-owned app.css left by the compose dev container can't be opened in place.
	@rm -f $(CSS_OUT)
	@$(TAILWIND) -c assets/tailwind/tailwind.config.js -i assets/tailwind/styles.css -o $(CSS_OUT) --minify

generate:
	@echo "INFO: Generating templ components"
	@templ generate
	@find . -name '*_templ.go' -not -path './.git/*' -exec gofmt -w {} +

templ-diff: generate
	@echo "INFO: Checking templ generated code is up to date"
	@if [ -n "$$(git status --porcelain -- '*_templ.go')" ]; then \
		echo "ERROR: templ generated code is out of date, run make generate:"; \
		git status --porcelain -- '*_templ.go'; \
		exit 1; \
	fi

mocks:
	@echo "INFO: Generating mocks"
	@mockery

docker:
	@echo "INFO: Building bot Docker image"
	@docker build --target bot --build-arg TAG="${TAG}" -t "${REGISTRY}/${APP}:${TAG}" -f Dockerfile .

docker-web:
	@echo "INFO: Building web Docker image"
	@docker build --target web --build-arg TAG="${TAG}" -t "${REGISTRY}/${WEB_APP}:${TAG}" -f Dockerfile .

push-to-registry:
	@docker push "${REGISTRY}/${APP}:${TAG}"
	@echo "INFO: Pushed ${REGISTRY}/${APP}:${TAG}"

push-web-to-registry:
	@docker push "${REGISTRY}/${WEB_APP}:${TAG}"
	@echo "INFO: Pushed ${REGISTRY}/${WEB_APP}:${TAG}"

sqlc-diff:
	@echo "INFO: Running sqlc diff"
	@sqlc diff -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/worldname/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/guild/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/webuser/postgresql/sqlc.yaml

migrations-validate:
	@echo "INFO: Validating migrations"
	@atlas migrate validate --dir "file://internal/infrastructure/db/postgresql/migrations"

migrations-hash:
	@echo "INFO: Hashing migrations"
	@atlas migrate hash --dir "file://internal/infrastructure/db/postgresql/migrations"

test: install-dependencies sqlc-diff lint css
	@echo "INFO: Running tests"
	@go test -cover -race -coverprofile=coverage.out ./...

test-coverage: test
	@echo "INFO: Generating test coverage report"
	@go tool cover -html=coverage.out

# Run go vet
go-vet:
	@echo "INFO: Running go vet"
	@go vet ./...

# Check formatting
fmt-check:
	@echo "INFO: Checking formatting"
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "ERROR: The following files are not formatted:"; \
		gofmt -l .; \
		exit 1; \
	fi

# Check for high cyclomatic complexity
gocyclo:
	@echo "INFO: Running gocyclo"
	@output=$$(gocyclo -over 15 -ignore '_templ\.go$$' .) ; \
	if [ "$$output" != "" ]; then \
		echo "Gocyclo complexity complaints:"; \
		echo "$$output"; \
		exit 1; \
	fi

staticcheck:
	@echo "INFO: Running staticcheck"
	@staticcheck ./...

# Run golint across the codebase
lint: install-dependencies
	@echo "INFO: Running lint"

	@make -s templ-diff fmt-check go-vet gocyclo staticcheck

sqlc-generate:
	@echo "INFO: Generating sqlc"
	@sqlc generate -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/worldname/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/guild/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/webuser/postgresql/sqlc.yaml

sqlc-vet:
	@echo "INFO: Running sqlc vet"
	@sqlc vet -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/worldname/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/guild/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/webuser/postgresql/sqlc.yaml

build: install-dependencies sqlc-generate test
	@make build-only

build-only:
	@echo "INFO: Building version: ${TAG}"
	@CGO_ENABLED=0 go build -o ./bin/${BOT_BIN} -ldflags="${LDFLAGS}" ./cmd/bot
	@CGO_ENABLED=0 go build -o ./bin/${WEB_BIN} -ldflags="${LDFLAGS}" ./cmd/web

bench: install-dependencies go-vet gocyclo
	@echo "INFO: Running benchmarks"
	@go test -bench='.' -benchmem ./...> _bench.out
	benchstat _bench.out
	@rm _bench.out

bench-long: install-dependencies go-vet gocyclo
	@echo "INFO: Running long benchmarks. Go make a coffee!"
	@go test -bench='.' -count=6 -timeout=5m -benchmem ./...> _bench.out
	benchstat _bench.out
	@rm _bench.out