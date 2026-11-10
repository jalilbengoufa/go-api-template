BINARY      := api
CMD_PATH    := ./cmd/api
BUILD_DIR   := bin
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GOLANGCI    := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

.DEFAULT_GOAL := help

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## Sync go.mod and go.sum
	go mod tidy

.PHONY: build
build: ## Compile the API binary
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)

.PHONY: run
run: ## Run the API from source
	go run $(CMD_PATH)

.PHONY: fmt
fmt: ## Format the code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI) run ./...

.PHONY: test
test: ## Run tests with the race detector
	go test -race -shuffle=on ./...

.PHONY: cover
cover: ## Report test coverage
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: cover-html
cover-html: cover ## Open the HTML coverage report
	go tool cover -html=coverage.out -o coverage.html

.PHONY: check
check: tidy vet lint test ## Run every quality gate

.PHONY: docker-build
docker-build: ## Build the container image
	docker build --build-arg VERSION=$(VERSION) -t $(BINARY):$(VERSION) .

.PHONY: up
up: ## Start local dependencies
	docker compose up -d

.PHONY: down
down: ## Stop local dependencies
	docker compose down -v

.PHONY: clean
clean: ## Remove build and coverage artifacts
	rm -rf $(BUILD_DIR) coverage.out coverage.html
