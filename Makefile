# CLASP Makefile

.PHONY: build run test clean install

# Build variables
BINARY_NAME=clasp
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Build the binary
build:
	@echo "Building CLASP..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/clasp

# Run the proxy
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Install to $GOPATH/bin
install: build
	go install ./cmd/clasp

# Install to ~/.clasp/bin
install-local: build
	@mkdir -p $(HOME)/.clasp/bin
	cp $(BUILD_DIR)/$(BINARY_NAME) $(HOME)/.clasp/bin/
	@echo "Installed to $(HOME)/.clasp/bin/$(BINARY_NAME)"
	@echo "Add $(HOME)/.clasp/bin to your PATH"

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod tidy
	go mod download

# Development run with auto-reload (requires air)
dev:
	air -c .air.toml

# Help
help:
	@echo "CLASP - Claude Language Agent Super Proxy"
	@echo ""
	@echo "Available targets:"
	@echo "  build         Build the binary"
	@echo "  run           Build and run"
	@echo "  test          Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  clean         Remove build artifacts"
	@echo "  install       Install to GOPATH/bin"
	@echo "  install-local Install to ~/.clasp/bin"
	@echo "  fmt           Format code"
	@echo "  lint          Lint code"
	@echo "  deps          Download dependencies"
	@echo "  dev           Run with hot reload (requires air)"
