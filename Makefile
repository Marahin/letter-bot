GREEN=\033[0;32m
RESET=\033[0m

APP ?= spot-assistant-bot
TAG ?= $(shell git rev-parse --short HEAD)
REGISTRY ?= registry.marahin.pl

.PHONY: install-dependencies install-bins go-mod

install-bins:
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.26.0

go-mod:
	@echo "$(GREEN)INFO: Running go mod tmakefileidy$(RESET)"
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

test: install-dependencies sqlc-diff go-vet gocyclo
	@echo "$(GREEN)INFO: Running tests$(RESET)"
	@go test -race -coverprofile=coverage.out ./...

test-coverage: test
	@echo "$(GREEN)INFO: Generating test coverage report$(RESET)"
	@go tool cover -html=coverage.out

go-vet:
	@echo "$(GREEN)INFO: Running go vet$(RESET)"
	@go vet ./...

gocyclo:
	@echo "$(GREEN)INFO: Running gocyclo$(RESET)"
	@output=$$(gocyclo -over 15 .) ; \
	if [ $$? -ne 0 ]; then \
		echo "Gocyclo complexity complaints: "; \
		echo $$output; \
		exit 1; \
	fi
	
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