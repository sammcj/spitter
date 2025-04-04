# Makefile for Spitter

# Variables
BINARY_NAME=spitter
MAIN_PATH=./cmd/spitter
GO=go
GOFMT=gofmt
GOFLAGS=-v

# Build settings
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
.PHONY: all
all: clean build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Install the binary
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install $(GOFLAGS) $(LDFLAGS) $(MAIN_PATH)
	@echo "Installation complete"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) -w .
	@echo "Formatting complete"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test -v ./...
	@echo "Tests complete"

# Build for multiple platforms
.PHONY: release
release: clean
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "Release builds complete"

# Help target
.PHONY: help
help:
	@echo "Spitter Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all      - Clean and build the binary"
	@echo "  build    - Build the binary"
	@echo "  clean    - Remove build artifacts"
	@echo "  install  - Install the binary"
	@echo "  fmt      - Format the code"
	@echo "  test     - Run tests"
	@echo "  release  - Build binaries for multiple platforms"
	@echo "  help     - Show this help message"
