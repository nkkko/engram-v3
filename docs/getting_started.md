# Getting Started with Engram v3

This guide will walk you through the initial steps to get Engram v3 up and running.

## Prerequisites

- Go 1.21+ or Rust 1.76+
- Protocol Buffers compiler
- Docker (optional, for containerized deployment)
- 2GB RAM minimum (4GB+ recommended)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/nkkko/engram-v3.git
cd engram-v3

# Install dependencies
go mod download

# Build the project
go build -o engram ./cmd/engram/main.go

# Run the server
./engram
```

### Docker Installation

For containerized deployment:

```bash
# Clone the repository
git clone https://github.com/nkkko/engram-v3.git
cd engram-v3

# Build the Docker image
docker build -t engram:v3 .

# Run the container
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  engram:v3
```

## Configuration

Engram v3 can be configured using environment variables or command-line arguments:

```bash
# Using environment variables
export LOG_LEVEL=debug
export MAX_CONNECTIONS=1000
export VECTOR_SEARCH_ENABLED=true
export WEAVIATE_URL=http://weaviate:8080
./engram

# Or using command-line arguments
./engram --addr=:8080 --data-dir=/app/data
```

Common configuration options:

| Option | Description | Default |
|--------|-------------|---------|
| `LOG_LEVEL` | Logging verbosity (debug, info, warn, error) | info |
| `VECTOR_SEARCH_ENABLED` | Enable vector search capabilities | false |
| `WEAVIATE_URL` | URL for Weaviate vector database | http://localhost:8080 |
| `TELEMETRY_ENABLED` | Enable telemetry data collection | true |
| `STORAGE_OPTIMIZED` | Enable storage optimizations | true |

## Verifying Installation

To verify your installation is working correctly:

```bash
# Check the health endpoint
curl http://localhost:8080/healthz

# Expected response: OK
```

## Your First API Request

Let's create a simple work unit to verify everything is working:

```bash
# Create a work unit
curl -X POST http://localhost:8080/workunit \
  -H "Content-Type: application/json" \
  -H "X-Agent-ID: example-agent" \
  -d '{
    "context_id": "ctx-123",
    "type": 1,
    "payload": "SGVsbG8gd29ybGQ="
  }'

# Retrieve the work unit
curl -X GET "http://localhost:8080/workunits/ctx-123" \
  -H "X-Agent-ID: example-agent"
```

## Using the MCP Server with Claude Desktop

If you want to use Engram v3 with Claude Desktop for enhanced conversation memory:

1. Build the MCP server:
   ```bash
   make build-mcp
   ```

2. Configure and start the MCP server:
   ```bash
   ./bin/mcp-server --config=./config/mcp-config.yaml
   ```

3. Configure Claude Desktop to use the MCP server (see [Claude Desktop Setup Guide](../apps/mcp/claude-desktop-setup.md) for details).

## Next Steps

- Explore the [API Documentation](api.md) for a complete reference of available endpoints.
- Learn about [Vector Search](vector_search.md) capabilities for semantic search.
- Check out the [Deployment Guide](deployment.md) for production deployment options.
- Review [Health Checks and Monitoring](health_checks.md) for operational best practices.
- See [Examples](../examples/) directory for code samples and use cases.
