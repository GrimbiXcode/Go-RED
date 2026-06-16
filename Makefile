# Go—RED Makefile
.PHONY: all build run test clean deps

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary name
BINARY_NAME=go-red

# Directories
BIN_DIR=bin
CMD_DIR=cmd/go-red

# Build flags
LDFLAGS=

# Version
VERSION?=dev

all: build

build: go.mod
	@echo "Building Go—RED..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) -v $(LDFLAGS) ./$(CMD_DIR)
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

run: go.mod
	@echo "Running Go—RED..."
	$(GOCMD) run ./$(CMD_DIR)

test: go.mod
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) -v ./...
	$(GOMOD) tidy

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Build WebUI
build-web:
	@echo "Building WebUI..."
	cd web && npm install && npm run build

# Build everything
build-all: build build-web

# Run with hot reload (requires air)
run-dev:
	@echo "Running with hot reload..."
	air -c .air.toml

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t ghcr.io/grimbixcode/go-red:latest .
	docker build -t ghcr.io/grimbixcode/go-red:$(VERSION) .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 -v $(PWD)/data:/app/data -v $(PWD)/plugins:/app/plugins ghcr.io/grimbixcode/go-red:latest

# Clean Docker
docker-clean:
	@echo "Cleaning Docker..."
	docker system prune -f

help:
	@echo "Available commands:"
	@echo "  make all       - Build the application"
	@echo "  make run       - Run the application"
	@echo "  make test      - Run tests"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make deps      - Download dependencies"
	@echo "  make fmt       - Format code"
	@echo "  make build-web - Build WebUI"
