.PHONY: build run clean test install deps fmt lint

# Binary name
BINARY_NAME=alpaca-tui
BINARY_PATH=./bin/$(BINARY_NAME)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint
GOPATH=$(shell go env GOPATH)

# Build flags
LDFLAGS=-ldflags "-s -w"

# Default target
all: test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) -v

# Build for multiple platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 -v

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 -v
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 -v

build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe -v

# Run the application
run: build
	$(BINARY_PATH)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf bin/

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BINARY_PATH) $(GOPATH)/bin/$(BINARY_NAME)

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Run linter
lint:
	@echo "Running linter..."
	$(GOLINT) run

# Development mode - rebuild on changes (requires entr)
watch:
	@echo "Watching for changes..."
	find . -name '*.go' | entr -r make run

# Quick commands for common operations
quote:
	$(BINARY_PATH) q AAPL

account:
	$(BINARY_PATH) acct

positions:
	$(BINARY_PATH) pos

orders:
	$(BINARY_PATH) orders



# CI workflow - replicates GitHub Actions
ci: deps-ci test-ci vet staticcheck build-test build-cross-platform

# Install dependencies and verify (CI style)
deps-ci:
	@echo "📦 Downloading dependencies..."
	$(GOMOD) download
	@echo "✅ Verifying dependencies..."
	$(GOMOD) verify

# Run tests with coverage (CI style)
test-ci: deps-ci
	@echo "�� Running tests with race detection and coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run go vet
vet:
	@echo "🔍 Running go vet..."
	$(GOCMD) vet ./...

# Run staticcheck
staticcheck:
	@echo "🔍 Running staticcheck..."
	@if ! command -v staticcheck &> /dev/null; then \
		echo "Installing staticcheck..."; \
		$(GOCMD) install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
	$(GOPATH)/bin/staticcheck ./...

# Build test
build-test: build
	@echo "�� Testing build..."
	$(BINARY_PATH) --help

# Cross-platform build test
build-cross-platform: build-linux build-darwin build-windows
	@echo "✅ Cross-platform builds completed"

help:
	@echo "Available targets:"
	@echo "  make build       - Build the binary"
	@echo "  make run         - Build and run the application"
	@echo "  make test        - Run tests"
	@echo "  make test-ci     - Run tests with coverage (CI style)"
	@echo "  make ci          - Run full CI workflow locally"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make install     - Install the binary to GOPATH/bin"
	@echo "  make deps        - Install/update dependencies"
	@echo "  make fmt         - Format code"
	@echo "  make lint        - Run linter"
	@echo "  make vet         - Run go vet"
	@echo "  make staticcheck - Run staticcheck"
	@echo "  make build-all   - Build for all platforms"
	@echo "  make watch       - Auto-rebuild on changes (requires entr)"