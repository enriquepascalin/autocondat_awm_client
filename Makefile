# AWM CLI Makefile
# Provides standardized targets for building, testing, and running the agent.

.PHONY: help build test run clean docker-build

# Variables
BINARY_NAME := awm-cli
BUILD_DIR   := bin
GO          := go
GOFLAGS     := -ldflags="-s -w"
DOCKER_IMAGE := awm-cli:latest

# Default target
help:
	@echo "Available targets:"
	@echo "  build        - Build the CLI binary"
	@echo "  test         - Run all unit tests"
	@echo "  run          - Build and run the CLI (requires configs/agent.yaml)"
	@echo "  clean        - Remove build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Build Docker image and run container"

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/awm-cli

# Run unit tests with race detection
test:
	$(GO) test -v -race -count=1 ./...

# Run the CLI (requires configuration)
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Run Docker container (mounts local configs directory)
docker-run: docker-build
	docker run --rm -it \
		-v $(PWD)/configs:/app/configs:ro \
		--network host \
		$(DOCKER_IMAGE)