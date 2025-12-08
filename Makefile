# CLASP Makefile

.PHONY: build run test clean install build-all release-binaries npm-publish docker docker-run docker-push

# Build variables
BINARY_NAME=clasp
BUILD_DIR=bin
DIST_DIR=dist
# Use git tag if on a tag, otherwise use the version from package.json
GIT_TAG=$(shell git describe --tags --exact-match 2>/dev/null)
PKG_VERSION=$(shell node -p "require('./package.json').version" 2>/dev/null || echo "dev")
VERSION=$(if $(GIT_TAG),$(GIT_TAG),v$(PKG_VERSION))
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Platforms for cross-compilation
PLATFORMS=darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

# Build the binary for current platform
build:
	@echo "Building CLASP..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/clasp

# Build binaries for all platforms
build-all:
	@echo "Building CLASP for all platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} ; \
		output_name=$(BINARY_NAME)-$$GOOS-$$GOARCH ; \
		if [ "$$GOOS" = "windows" ]; then output_name=$$output_name.exe; fi ; \
		echo "Building $$output_name..." ; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build $(LDFLAGS) -o $(DIST_DIR)/$$output_name ./cmd/clasp ; \
	done
	@echo "Build complete! Binaries in $(DIST_DIR)/"

# Upload release binaries to GitHub
release-binaries: build-all
	@echo "Uploading binaries to GitHub release v$(VERSION)..."
	@for file in $(DIST_DIR)/*; do \
		echo "Uploading $$file..." ; \
		gh release upload v$(VERSION) $$file --clobber ; \
	done
	@echo "Release binaries uploaded!"

# Publish to npm
npm-publish:
	@echo "Publishing to npm..."
	npm publish --access public
	@echo "Published to npm!"

# npm pack for testing
npm-pack:
	@echo "Creating npm package..."
	npm pack
	@echo "Package created!"

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

# Docker image name
DOCKER_IMAGE=clasp-ai
DOCKER_TAG=$(VERSION)

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -t $(DOCKER_IMAGE):latest .
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Run Docker container
docker-run:
	docker run -d --name clasp \
		-p 8080:8080 \
		-e PROVIDER=$${PROVIDER:-openai} \
		-e OPENAI_API_KEY=$$OPENAI_API_KEY \
		$(DOCKER_IMAGE):latest

# Stop Docker container
docker-stop:
	docker stop clasp 2>/dev/null || true
	docker rm clasp 2>/dev/null || true

# Push Docker image (requires docker login)
docker-push: docker
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) ghcr.io/jedarden/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker tag $(DOCKER_IMAGE):latest ghcr.io/jedarden/$(DOCKER_IMAGE):latest
	docker push ghcr.io/jedarden/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push ghcr.io/jedarden/$(DOCKER_IMAGE):latest
	@echo "Docker image pushed to ghcr.io/jedarden/$(DOCKER_IMAGE)"

# Build and run with docker-compose
compose-up:
	docker-compose up -d --build

# Stop docker-compose
compose-down:
	docker-compose down

# Help
help:
	@echo "CLASP - Claude Language Agent Super Proxy"
	@echo ""
	@echo "Available targets:"
	@echo "  build            Build the binary for current platform"
	@echo "  build-all        Build binaries for all platforms"
	@echo "  run              Build and run"
	@echo "  test             Run tests"
	@echo "  test-coverage    Run tests with coverage report"
	@echo "  clean            Remove build artifacts"
	@echo "  install          Install to GOPATH/bin"
	@echo "  install-local    Install to ~/.clasp/bin"
	@echo "  fmt              Format code"
	@echo "  lint             Lint code"
	@echo "  deps             Download dependencies"
	@echo "  dev              Run with hot reload (requires air)"
	@echo "  release-binaries Build and upload binaries to GitHub release"
	@echo "  npm-publish      Publish package to npm"
	@echo "  npm-pack         Create npm package tarball for testing"
	@echo "  docker           Build Docker image"
	@echo "  docker-run       Run Docker container"
	@echo "  docker-stop      Stop Docker container"
	@echo "  docker-push      Push Docker image to GHCR"
	@echo "  compose-up       Start with docker-compose"
	@echo "  compose-down     Stop docker-compose"
