.PHONY: build clean test run docker-build docker-run docker-test proto

# Variables
BINARY_NAME=engram
BUILD_DIR=./bin
PROTO_DIR=./proto
PROTO_OUT=./pkg/proto
VERSION=$(shell git describe --tags --always || echo "dev")
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.Commit=${COMMIT}"
DOCKER_IMAGE=engram
DOCKER_TAG=$(VERSION)

# Go commands
GO=go
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOTEST=$(GO) test
GOGET=$(GO) get

# Default target
all: test build

# Build the application
build:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/engram

# Build with race detection
build-race:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -race -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/engram

# Clean build files
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(PROTO_OUT)

# Run tests
test:
	$(GOTEST) -v ./...

# Run benchmarks
bench:
	$(GOTEST) -bench=. ./...

# Run the application
run:
	$(GO) run $(LDFLAGS) ./cmd/engram

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Run Docker container
docker-run:
	docker run -p 8080:8080 -v $(PWD)/data:/app/data $(DOCKER_IMAGE):$(DOCKER_TAG)
	
# Run tests in Docker
docker-test:
	docker build --target test -t $(DOCKER_IMAGE):test .
	docker run --rm $(DOCKER_IMAGE):test

# Run specific test in Docker
docker-test-%:
	docker build --target test -t $(DOCKER_IMAGE):test .
	docker run --rm $(DOCKER_IMAGE):test go test -v ./$* -count=1

# Generate Protocol Buffers
proto:
	mkdir -p $(PROTO_OUT)
	protoc --go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/*.proto

# Download dependencies
deps:
	$(GOGET) -v ./...

# Format code
fmt:
	$(GO) fmt ./...

# Lint code
lint:
	golangci-lint run ./...

# Lint and fix
lint-fix:
	golangci-lint run --fix ./...

# Generate 
generate:
	$(GO) generate ./...

# Install binary
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

# Version info
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Commit: $(COMMIT)"

# MCP Server targets
build-mcp:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/mcp-server ./apps/mcp

run-mcp: build-mcp
	$(BUILD_DIR)/mcp-server

test-mcp:
	cd apps/mcp/cmd/test-client && $(GO) run main.go
	
test-mcp-claude: build-mcp
	cd apps/mcp/cmd/claude-test && $(GO) run main.go