APP ?= spot-assistant-bot
TAG ?= $(shell git rev-parse --short HEAD)
REGISTRY ?= registry.marahin.pl

.PHONY: install-dependencies

install-dependencies:
	go mod download

docker:
	docker build -t "${REGISTRY}/${APP}:${TAG}" -f Dockerfile .

push-to-registry:
	docker push "${REGISTRY}/${APP}:${TAG}"
	echo "${REGISTRY}/${APP}:${TAG}"

sqlc-diff:
	sqlc diff -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	sqlc diff -f internal/infrastructure/spot/postgresql/sqlc.yaml

test: sqlc-diff go-vet
	go test -race -coverprofile=coverage.out ./...

test-coverage: test
	go tool cover -html=coverage.out	

go-vet: 
	go vet ./...

sqlc-generate:
	sqlc generate -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	sqlc generate -f internal/infrastructure/spot/postgresql/sqlc.yaml

sqlc-vet:
	sqlc vet -f internal/infrastructure/reservation/postgresql/sqlc.yaml
	sqlc vet -f internal/infrastructure/spot/postgresql/sqlc.yaml

build: install-dependencies sqlc-generate test
	make build-only

build-only:
	echo "My tag is: ${TAG}"
	CGO_ENABLED=0 go build -o ${APP} -ldflags="-X spot-assistant/internal/common/version.Version=${TAG}" cmd/main.go