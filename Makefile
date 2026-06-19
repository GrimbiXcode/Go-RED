# Go—RED Makefile
.PHONY: all build run test clean deps

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOPATH=$(HOME)/go

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
	@echo "Running Go backend tests..."
	$(GOTEST) -v -race ./...

# Run frontend tests
test-frontend:
	@echo "Running frontend tests..."
	cd web && npm test

# Run all tests
test-all: test test-frontend

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

# Run frontend dev server
run-frontend:
	@echo "Starting frontend dev server..."
	cd web && npm run dev

# Run with hot reload (requires air)
run-dev:
	@echo "Starting both frontend and backend dev servers..."
	@echo "To see frontend logs: tail -f /tmp/frontend.log"
	@echo "To stop: make stop-dev"
	cd web && npm run dev > /tmp/frontend.log 2>&1 & \
	$(GOPATH)/bin/air -c .air.toml

# Stop both dev servers
stop-dev:
	@echo "Stopping frontend and backend dev servers..."
	@pkill -f 'npm run dev' || true
	@pkill -f 'air' || true
	@echo "Dev servers stopped"

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
	@echo "  make all          - Build the application"
	@echo "  make run          - Run the application"
	@echo "  make run-dev      - Run both backend and frontend dev servers"
	@echo "  make run-frontend - Run only frontend dev server"
	@echo "  make stop-dev     - Stop both backend and frontend dev servers"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make deps         - Download dependencies"
	@echo "  make fmt          - Format code"
	@echo "  make build-web    - Build WebUI"
