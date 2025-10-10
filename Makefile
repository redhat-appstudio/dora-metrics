# DORA Metrics Server Makefile

# Version information
# For Go Releaser: use the current git tag, fallback to commit-dirty
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -X main.BuildVersion=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.BuildCommit=$(BUILD_COMMIT)

# Binary name and directory
BINARY := dora-metrics
BIN_DIR := bin

# Default target
.PHONY: all
all: build

# Build the application for current platform
.PHONY: build
build:
	@echo "Building $(BINARY) for current platform..."
	@echo "Version: $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Build commit: $(BUILD_COMMIT)"
	@echo "Platform: $(shell go env GOOS)/$(shell go env GOARCH)"
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/server
	@echo "Build completed successfully!"

# Build for multiple platforms (macOS ARM, macOS Intel, Linux)
.PHONY: build-all
build-all:
	@echo "Building for multiple platforms..."
	@echo "Version: $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Build commit: $(BUILD_COMMIT)"
	@mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/dora-metrics-macos-arm ./cmd/server
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/dora-metrics-macos-intel ./cmd/server
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/dora-metrics-linux-arm ./cmd/server
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/dora-metrics-linux-intel ./cmd/server
	@echo "Multi-platform build completed successfully!"
	@ls -la dist/

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BIN_DIR)/$(BINARY)
	rm -rf dist/

# Run the application
.PHONY: run
run: build
	./$(BIN_DIR)/$(BINARY)

# Run with help
.PHONY: help
help: build
	./$(BIN_DIR)/$(BINARY) -help

# Run with version
.PHONY: version
version: build
	./$(BIN_DIR)/$(BINARY) -version

# Install dependencies
.PHONY: deps
deps:
	go mod tidy
	go mod download

# Run tests
.PHONY: test
test:
	go test ./...

# Run unit tests with coverage at project level
.PHONY: unit-test
unit-test:
	@echo "Running unit tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo ""
	@echo "Coverage report saved to coverage.out"
	@echo "To view HTML coverage report, run: go tool cover -html=coverage.out"

# Run linter
.PHONY: lint
lint:
	golangci-lint run

# Show build information
.PHONY: info
info:
	@echo "Version: $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"
	@echo "Build commit: $(BUILD_COMMIT)"
	@echo "Go version: $(shell go version)"
