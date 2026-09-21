# Go—RED Makefile
#
# New here? `make start` is all you need: it checks your tools, installs the
# editor's dependencies, builds everything and starts Go—RED.
# `make help` lists the rest.
#
# Targets carry a `## description` that `make help` prints; a `##@ Title`
# line starts a new group in that output.

.DEFAULT_GOAL := help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod

# Binary name
BINARY_NAME=go-red

# Directories
BIN_DIR=bin
CMD_DIR=cmd/go-red
WEB_DIR=web
DIST_DIR=internal/webui/dist

# Runtime settings, e.g. `make start PORT=9090`
PORT?=8080
DATA_DIR?=data

# Version, stamped into the binary the way the release build does it
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Minimum tool versions (Go follows go.mod, Node follows CI)
GO_MIN_MINOR=$(shell sed -n 's/^go 1\.\([0-9]*\).*/\1/p' go.mod)
NODE_MIN_MAJOR=22

# Hot reload for the backend. Looked up in Go's bin directory only: a bare
# `air` on PATH may be something else entirely (JetBrains ships an `air` too).
AIR=$(shell $(GOCMD) env GOPATH 2>/dev/null)/bin/air

# Stamp files: make reinstalls or rebuilds only when an input is newer.
# npm writes node_modules/.package-lock.json itself on every install.
WEB_DEPS=$(WEB_DIR)/node_modules/.package-lock.json
WEB_DIST=$(DIST_DIR)/index.html
WEB_SOURCES=$(shell find $(WEB_DIR)/src $(WEB_DIR)/public -type f 2>/dev/null) \
	$(wildcard $(WEB_DIR)/index.html $(WEB_DIR)/*.config.* $(WEB_DIR)/tsconfig*.json)
GO_SOURCES=$(shell find cmd internal -type f -name '*.go' 2>/dev/null) go.mod go.sum

.PHONY: help start setup doctor all build build-web build-all run dev run-dev \
	run-frontend test test-frontend test-all e2e lint check fmt deps \
	generate-types check-types clean clean-all docker-build docker-run docker-clean

##@ Getting started

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make \033[36m<target>\033[0m\n"} \
		/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)} \
		/^[a-zA-Z0-9_-]+:.*## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "First time here? Run: make start"

start: doctor $(BIN_DIR)/$(BINARY_NAME) ## Check tools, install, build and run Go—RED on http://localhost:8080
	@echo ""
	@echo "Go—RED is starting on http://localhost:$(PORT)  (Ctrl+C stops it)"
	@echo ""
	$(BIN_DIR)/$(BINARY_NAME) -port $(PORT) -data-dir $(DATA_DIR)

setup: doctor $(WEB_DEPS) ## Install all dependencies (Go modules and the editor's npm packages)
	$(GOMOD) download
	@echo "Setup complete. Next: make start (run it) or make dev (work on it)"

doctor: ## Check that Go, Node and npm are installed and new enough
	@ok=1; \
	if command -v $(GOCMD) >/dev/null 2>&1; then \
		v=$$($(GOCMD) env GOVERSION | sed 's/^go//'); minor=$$(echo $$v | cut -d. -f2); \
		if [ "$$minor" -ge "$(GO_MIN_MINOR)" ] 2>/dev/null; then echo "  ok  Go $$v"; \
		else echo "  !!  Go $$v is too old, need 1.$(GO_MIN_MINOR)+  -> https://go.dev/dl/"; ok=0; fi; \
	else echo "  !!  Go is not installed, need 1.$(GO_MIN_MINOR)+  -> https://go.dev/dl/"; ok=0; fi; \
	if command -v node >/dev/null 2>&1; then \
		v=$$(node --version | sed 's/^v//'); major=$$(echo $$v | cut -d. -f1); \
		if [ "$$major" -ge "$(NODE_MIN_MAJOR)" ] 2>/dev/null; then echo "  ok  Node $$v"; \
		else echo "  !!  Node $$v is too old, need $(NODE_MIN_MAJOR)+  -> https://nodejs.org/"; ok=0; fi; \
	else echo "  !!  Node is not installed, need $(NODE_MIN_MAJOR)+  -> https://nodejs.org/"; ok=0; fi; \
	if command -v npm >/dev/null 2>&1; then echo "  ok  npm $$(npm --version)"; \
	else echo "  !!  npm is not installed (it ships with Node)  -> https://nodejs.org/"; ok=0; fi; \
	if [ $$ok -eq 0 ]; then echo ""; echo "Install the tools marked !! and run the command again."; \
		echo "No Go or Node? 'make docker-build docker-run' only needs Docker."; exit 1; fi

##@ Build

all: build-all

build: go.mod ## Build the binary with whatever editor build is present (fast, Go only)
	@echo "Building Go—RED..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) $(LDFLAGS) ./$(CMD_DIR)
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

build-web: $(WEB_DIST) ## Build the editor into internal/webui/dist, where the Go binary embeds it

build-all: $(BIN_DIR)/$(BINARY_NAME) ## Build the editor, then the binary that embeds it

# Reinstall only when the lockfile changed (or node_modules is missing).
$(WEB_DEPS): $(WEB_DIR)/package.json $(WEB_DIR)/package-lock.json
	@echo "Installing editor dependencies..."
	cd $(WEB_DIR) && npm ci --no-audit --no-fund
	@touch $@

# Rebuild the editor only when one of its sources changed.
$(WEB_DIST): $(WEB_DEPS) $(WEB_SOURCES)
	@echo "Building WebUI..."
	cd $(WEB_DIR) && npm run build
	@touch $@

# The binary embeds the editor, so it follows both source trees.
$(BIN_DIR)/$(BINARY_NAME): $(WEB_DIST) $(GO_SOURCES)
	@echo "Building Go—RED..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $@ $(LDFLAGS) ./$(CMD_DIR)
	@echo "Build complete: $@"

##@ Run

run: go.mod $(WEB_DIST) ## Run from source with the editor embedded
	@echo "Running Go—RED on http://localhost:$(PORT) ..."
	$(GOCMD) run ./$(CMD_DIR) -port $(PORT) -data-dir $(DATA_DIR)

dev: go.mod $(WEB_DEPS) ## Develop: API on :8080 plus the editor with hot reload on http://localhost:5173
	@echo "Editor with hot reload: http://localhost:5173  (Ctrl+C stops both servers)"
	@(cd $(WEB_DIR) && exec npm run dev) & web=$$!; \
	trap 'kill $$web 2>/dev/null' INT TERM EXIT; \
	if [ -x "$(AIR)" ]; then "$(AIR)" -c .air.toml; \
	else echo "air not found, the backend runs without hot reload (go install github.com/air-verse/air@latest)"; \
		$(GOCMD) run ./$(CMD_DIR) -port $(PORT) -data-dir $(DATA_DIR); fi

run-dev: dev

run-frontend: $(WEB_DEPS) ## Run only the editor's dev server
	@echo "Starting frontend dev server..."
	cd $(WEB_DIR) && npm run dev

##@ Test and check

test: go.mod ## Run Go tests with the race detector
	@echo "Running Go backend tests..."
	$(GOTEST) -race ./...

test-frontend: $(WEB_DEPS) ## Run the editor's unit tests once
	@echo "Running frontend tests..."
	cd $(WEB_DIR) && npx vitest run

test-all: test test-frontend ## Run Go and editor tests

e2e: $(WEB_DEPS) ## Run the Playwright end-to-end tests against the real server
	cd $(WEB_DIR) && npm run e2e:full

# Static checks for the Go side (what CI runs).
lint: go.mod ## gofmt + go vet
	@echo "Checking gofmt..."
	@UNFORMATTED=$$(gofmt -l .); if [ -n "$$UNFORMATTED" ]; then echo "$$UNFORMATTED"; exit 1; fi
	$(GOCMD) vet ./...

check: lint test check-types $(WEB_DEPS) ## Everything CI checks, locally: Go and web
	cd $(WEB_DIR) && npm run typecheck && npm run lint && npx vitest run && npm run build

fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

##@ Maintenance

deps: ## Download Go modules and tidy go.mod
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Regenerate web/src/types/generated.ts from internal/dto, internal/registry,
# and cmd/go-red/websocket. Run this after changing any of those packages.
generate-types: go.mod ## Regenerate the TypeScript types from the Go DTOs
	@echo "Generating TypeScript types from Go DTOs..."
	$(GOCMD) generate ./internal/dto/...

# Fail if generated.ts is stale relative to the Go DTOs (used in CI).
check-types: generate-types
	@git diff --exit-code -- web/src/types/generated.ts || \
		(echo "web/src/types/generated.ts is stale -- run 'make generate-types' and commit the result" && exit 1)

clean: ## Remove build artifacts (binary and built editor)
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	find $(DIST_DIR) -mindepth 1 ! -name .gitkeep -exec rm -rf {} +

clean-all: clean ## Also remove the editor's node_modules
	rm -rf $(WEB_DIR)/node_modules

##@ Docker (needs neither Go nor Node)

docker-build: ## Build the Docker image
	@echo "Building Docker image..."
	docker build -t ghcr.io/grimbixcode/go-red:latest -t ghcr.io/grimbixcode/go-red:$(VERSION) .

docker-run: ## Run the Docker image on http://localhost:8080
	@echo "Running Docker container..."
	docker run --rm -p $(PORT):8080 -v $(PWD)/data:/app/data -v $(PWD)/plugins:/app/plugins ghcr.io/grimbixcode/go-red:latest

docker-clean: ## Prune unused Docker data (affects ALL of Docker, not just Go—RED)
	@echo "Cleaning Docker..."
	docker system prune -f
