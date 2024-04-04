GREEN=\033[0;32m
RESET=\033[0m

APP ?= spot-assistant-bot
TAG ?= $(shell git rev-parse --short HEAD)
REGISTRY ?= registry.marahin.pl

.PHONY: install-dependencies install-bins go-mod

install-bins:
	@for pkg in $$(go list -f '{{range .Imports}}{{.}} {{end}}' cmd/tools.go); do \
		echo "$(GREEN)INFO: Installing $$pkg$(RESET)"; \
		go install $$pkg; \
	done

go-mod:
	@echo "$(GREEN)INFO: Running go mod tidy$(RESET)"
	@go mod tidy

install-dependencies: install-bins go-mod
	@echo "$(GREEN)INFO: Downloading dependencies$(RESET)"

docker:
	@echo "$(GREEN)INFO: Building Docker image$(RESET)"
	@docker build -t "${REGISTRY}/${APP}:${TAG}" -f Dockerfile .

push-to-registry:
	@docker push "${REGISTRY}/${APP}:${TAG}"
	@echo "$(GREEN)INFO: Pushed ${REGISTRY}/${APP}:${TAG}$(RESET)"

sqlc-diff:
	@echo "$(GREEN)INFO: Running sqlc diff$(RESET)"
	@sqlc diff -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/spot/postgresql/sqlc.yaml

test: install-dependencies sqlc-diff go-vet
	@echo "$(GREEN)INFO: Running tests$(RESET)"
	@go test -race -coverprofile=coverage.out ./...

test-coverage: test
	@echo "$(GREEN)INFO: Generating test coverage report$(RESET)"
	@go tool cover -html=coverage.out

go-vet:
	@echo "$(GREEN)INFO: Running go vet$(RESET)"
	@go vet ./...

sqlc-generate:
	@echo "$(GREEN)INFO: Generating sqlc$(RESET)"
	@sqlc generate -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/spot/postgresql/sqlc.yaml

sqlc-vet:
	@echo "$(GREEN)INFO: Running sqlc vet$(RESET)"
	@sqlc vet -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/spot/postgresql/sqlc.yaml

build: install-dependencies sqlc-generate test
	@make build-only

build-only:
	@echo "$(GREEN)INFO: Building version: ${TAG}$(RESET)"
	@CGO_ENABLED=0 go build -o ${APP} -ldflags="-X spot-assistant/internal/common/version.Version=${TAG}" cmd/main.go