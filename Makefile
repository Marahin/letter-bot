APP ?= spot-assistant
TAG ?= $(shell git rev-parse --short HEAD)
REGISTRY ?= registry.marahin.pl

.PHONY: install-dependencies install-bins go-mod

install-bins:
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.26.0

go-mod:
	@echo "INFO: Running go mod tidy"
	@go mod tidy

install-dependencies: install-bins go-mod
	@echo "INFO: Downloading dependencies"

docker:
	@echo "INFO: Building Docker image"
	@docker build -t "${REGISTRY}/${APP}:${TAG}" -f Dockerfile .

push-to-registry:
	@docker push "${REGISTRY}/${APP}:${TAG}"
	@echo "INFO: Pushed ${REGISTRY}/${APP}:${TAG}"

sqlc-diff:
	@echo "INFO: Running sqlc diff"
	@sqlc diff -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc diff -f internal/infrastructure/guild/postgresql/sqlc.yaml

test: install-dependencies sqlc-diff go-vet gocyclo
	@echo "INFO: Running tests"
	@go test -cover -race -coverprofile=coverage.out ./...

test-coverage: test
	@echo "INFO: Generating test coverage report"
	@go tool cover -html=coverage.out

go-vet:
	@echo "INFO: Running go vet"
	@go vet ./...

gocyclo:
	@echo "INFO: Running gocyclo"
	@output=$$(gocyclo -over 15 .) ; \
	if [ $$? -ne 0 ]; then \
		echo "Gocyclo complexity complaints: "; \
		echo $$output; \
		exit 1; \
	fi
	
sqlc-generate:
	@echo "INFO: Generating sqlc"
	@sqlc generate -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc generate -f internal/infrastructure/guild/postgresql/sqlc.yaml

sqlc-vet:
	@echo "INFO: Running sqlc vet"
	@sqlc vet -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/spot/postgresql/sqlc.yaml
	@sqlc vet -f internal/infrastructure/guild/postgresql/sqlc.yaml

build: install-dependencies sqlc-generate test
	@make build-only

build-only:
	@echo "INFO: Building version: ${TAG}"
	@for dir in $$(ls cmd/); do \
		echo "INFO: Building cmd/$$dir"; \
		CGO_ENABLED=0 go build -o ./bin/${APP}-$$dir -ldflags="-X spot-assistant/internal/common/version.Version=${TAG}" cmd/$$dir/main.go; \
	done