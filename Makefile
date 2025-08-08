# Variables
BINARY_NAME_SERVER=nats-proxy-server
BINARY_NAME_CLIENT=nats-proxy-client
DOCKER_IMAGE=nats-proxy
DOCKER_TAG=latest
GO_VERSION=1.24
MODULE_NAME=github.com/igorrius/nats-proxy

# Build directories
BUILD_DIR=build
BIN_DIR=bin

# Go build flags
LDFLAGS=-ldflags "-s -w"
BUILD_FLAGS=-a -installsuffix cgo

# Default target
.PHONY: all
all: clean build

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build both client and server binaries"
	@echo "  build-server   - Build server binary"
	@echo "  build-client   - Build client binary"
	@echo "  test           - Run all tests"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Clean build artifacts"
	@echo "  fmt            - Format Go code"
	@echo "  lint           - Run golangci-lint"
	@echo "  vet            - Run go vet"
	@echo "  mod-tidy       - Tidy go modules"
	@echo "  mod-download   - Download go modules"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-clean   - Clean Docker images"
	@echo "  install        - Install binaries to GOPATH/bin"
	@echo "  run-server     - Run server locally"
	@echo "  run-client     - Run client locally"
	@echo "  deps           - Install development dependencies"

# Create directories
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# Build targets
.PHONY: build
build: build-server build-client

.PHONY: build-server
build-server: $(BIN_DIR)
	CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME_SERVER) ./cmd/server

.PHONY: build-client
build-client: $(BIN_DIR)
	CGO_ENABLED=0 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME_CLIENT) ./cmd/client

# Cross-compilation targets
.PHONY: build-linux
build-linux: $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME_SERVER)-linux-amd64 ./cmd/server
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME_CLIENT)-linux-amd64 ./cmd/client

.PHONY: build-windows
build-windows: $(BIN_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME_SERVER)-windows-amd64.exe ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME_CLIENT)-windows-amd64.exe ./cmd/client

.PHONY: build-darwin
build-darwin: $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME_SERVER)-darwin-amd64 ./cmd/server
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME_CLIENT)-darwin-amd64 ./cmd/client

.PHONY: build-all
build-all: build-linux build-windows build-darwin

# Test targets
.PHONY: test
test:
	go test -race -short ./...

.PHONY: test-verbose
test-verbose:
	go test -race -short -v ./...

.PHONY: test-coverage
test-coverage: $(BUILD_DIR)
	go test -race -coverprofile=$(BUILD_DIR)/coverage.out ./...
	go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report generated: $(BUILD_DIR)/coverage.html"

.PHONY: test-integration
test-integration:
	go test -race ./... -tags=integration

# Code quality targets
.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run 'make deps' to install it." && exit 1)
	golangci-lint run

.PHONY: check
check: fmt vet lint test

# Module management
.PHONY: mod-tidy
mod-tidy:
	go mod tidy

.PHONY: mod-download
mod-download:
	go mod download

.PHONY: mod-verify
mod-verify:
	go mod verify

# Docker targets
.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

.PHONY: docker-run
docker-run:
	docker run --rm -it \
		-e NATS_URL=nats://localhost:4222 \
		-e LOG_LEVEL=info \
		--name nats-proxy-server \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-run-detached
docker-run-detached:
	docker run -d \
		-e NATS_URL=nats://localhost:4222 \
		-e LOG_LEVEL=info \
		--name nats-proxy-server \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-stop
docker-stop:
	docker stop nats-proxy-server || true
	docker rm nats-proxy-server || true

.PHONY: docker-clean
docker-clean:
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) || true
	docker system prune -f

# Installation targets
.PHONY: install
install:
	go install $(LDFLAGS) ./cmd/server
	go install $(LDFLAGS) ./cmd/client

# Run targets
.PHONY: run-server
run-server:
	go run ./cmd/server --nats-url=nats://localhost:4222 --log-level=debug

.PHONY: run-client
run-client:
	go run ./cmd/client --nats-url=nats://localhost:4222 --log-level=debug

# Development dependencies
.PHONY: deps
deps:
	@echo "Installing development dependencies..."
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Development dependencies installed"

# Clean targets
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(BIN_DIR)
	go clean -cache
	go clean -testcache

.PHONY: clean-all
clean-all: clean docker-clean

# Release target
.PHONY: release
release: clean check build-all test-coverage
	@echo "Release build completed successfully"

# Development workflow
.PHONY: dev
dev: deps fmt vet test build

# CI/CD targets
.PHONY: ci
ci: mod-download check test-coverage build

.PHONY: cd
cd: ci docker-build
