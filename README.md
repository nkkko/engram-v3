# Engram v3

A real-time multi-agent collaboration engine with ultra-low latency communication and deterministic context handling.

> **Note:** We have migrated from RocksDB to Badger for the storage engine, simplifying deployment and improving performance.

## Overview

Engram v3 is a platform that enables multiple AI agents to collaborate effectively in real-time. It provides a shared workspace, communication bus, and memory system that prioritizes extremely low latency for interactions and state synchronization.

## Key Features

- Ultra-low-latency capture, store, query, and stream of "Work Units" (< 5 ms local)
- Deterministic context grouping and real-time fan-out to interested agents
- Pluggable locking/concurrency management for shared artifacts
- High-performance in-memory event queue with persistent storage
- Hybrid search capabilities (keyword and semantic)

## Architecture

Engram is built on several core design principles:

1. **Immutable Work Units**: All information is captured as immutable records, allowing for consistent state management.

2. **Event-Driven Architecture**: Internal components communicate via an event system for loose coupling and scaling.

3. **Context-Centric Organization**: Related work units are grouped into contexts, allowing agents to share workspaces.

4. **Append-Only Storage with Indexes**: The storage layer uses an append-only log with efficient indexes for fast retrieval.

5. **Real-Time Fan-Out**: Changes are broadcast to interested agents with minimal latency.

The system is built using a modular architecture with these key components:

- **API Gateway**: High-performance HTTP/gRPC interface
- **Event Router**: Manages work unit distribution
- **Storage Engine**: Uses Badger (pure Go key-value store) with WAL for durability and performance
- **Notifier**: WebSocket/SSE hub for real-time updates
- **Lock Manager**: Handles resource access control
- **Search Index**: Provides fast retrieval capabilities

## Getting Started

### Prerequisites

- Go 1.21+ or Rust 1.76+
- Protocol Buffers compiler

### Installation

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

### Docker Setup

For containerized deployment:

```bash
# Build the Docker image
make docker-build

# Run the container
make docker-run
```

### Running Tests with Docker

You can run tests in a containerized environment:

```bash
# Run all tests (core, search, storage, and api)
make docker-test

# Run tests in a specific package
make docker-test-internal/lockmanager

# Run tests in multiple specific packages
make docker-test-internal/router docker-test-internal/metrics
```

The Docker test environment is configured to run:
- Core tests (lockmanager, router, metrics, notifier) which always work reliably
- SQLite-dependent tests (search) - may require additional configuration for FTS5 
- Badger storage tests (storage) - fully native Go implementation

The test script will run all packages and report any failures, ensuring that even if one dependency has issues, other tests will still run. This gives you a comprehensive report of what's working and what needs attention.

Note: For full compatibility with all tests, a more specialized environment might be needed. The Docker setup provides a solid baseline for the core functionality testing.

## Storage Engine

Engram uses [Badger](https://github.com/hypermodeinc/badger) as its storage engine, which provides several advantages:

- **Pure Go Implementation**: No CGO dependencies, simplifying deployment and cross-compilation
- **Optimized for SSDs**: Badger's architecture separates keys from values to reduce write amplification
- **Transactional**: Supports fully ACID transactions with serializable snapshot isolation
- **High Performance**: Designed for high read and write throughput
- **Key Prefixing**: Simulates column families through a key prefixing scheme for logical data separation

The storage layer is built with the following features:

- **Write-Ahead Log (WAL)**: Ensures durability of operations even in case of crashes
- **Efficient Batching**: Optimizes write throughput with configurable batch sizes
- **In-Memory Caching**: Optional caching layer for frequently accessed items
- **Prefix-Based Organization**: Organizes data by type using key prefixes instead of column families
- **Transactional Operations**: All operations are transaction-based for consistency

## Configuration

Engram can be configured via command-line flags, environment variables, or a configuration file. For a full list of configuration options, see the example configuration file:

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your settings
```

Key configuration options:

```bash
# Using command-line flags
./engram --addr=:8080 --data-dir=./data --log-level=debug

# Using environment variables (not yet implemented)
export ENGRAM_ADDR=:8080
export ENGRAM_DATA_DIR=./data
./engram
```

## API Surface

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/workunit` | POST | Create a new work unit |
| `/workunit/{id}` | GET | Retrieve a specific work unit |
| `/workunits/{contextId}` | GET | List work units for a context |
| `/contexts/{id}` | PATCH | Modify a context |
| `/contexts/{id}` | GET | Get a context |
| `/stream?context={id}` | WebSocket | Stream updates in real-time |
| `/stream-sse?context={id}` | SSE | Stream updates using Server-Sent Events |
| `/locks` | POST | Acquire a lock on a resource |
| `/locks/{resource}` | DELETE | Release a lock |
| `/locks/{resource}` | GET | Get lock information |
| `/search` | POST | Search for work units |

## Performance

Engram v3 is designed for ultra-low latency operations:

- Single WorkUnit write: ≤ 1 ms p99
- Stream fan-out latency: ≤ 5 ms end-to-end
- Query of last 100 units: ≤ 2 ms

## Example Usage

```go
package main

import (
    "fmt"
    "github.com/nkkko/engram-v3/client"
)

func main() {
    // Create a client
    c := client.New("http://localhost:8080")
    
    // Create a context
    ctx, err := c.CreateContext("Example Context")
    if err != nil {
        panic(err)
    }
    
    // Create a work unit
    unit, err := c.CreateWorkUnit(client.WorkUnit{
        ContextID: ctx.ID,
        AgentID:   "agent-1",
        Type:      "MESSAGE",
        Payload:   "Hello, Engram!",
    })
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Created work unit: %s\n", unit.ID)
    
    // Subscribe to updates
    subscription, err := c.Subscribe(ctx.ID)
    if err != nil {
        panic(err)
    }
    
    for event := range subscription.Events {
        fmt.Printf("Received: %+v\n", event)
    }
}
```

For more examples, see the [examples](./examples) directory.

## Development

```bash
# Run all tests with the test script
./test.sh

# Run specific tests
go test ./internal/storage
go test ./internal/api
go test ./internal/search

# Run tests in Docker with properly configured environment
docker build --target test -t engram-v3:test .
docker run --rm engram-v3:test

# Run benchmarks
go test -bench=. ./internal/storage

# Generate protobuf code (if protoc is installed)
protoc --go_out=. --go_opt=paths=source_relative proto/engram.proto
```

## Using as a Git Submodule

Engram v3 can be used as a Git submodule in other projects. This is especially useful when integrating with tools like Codex CLI that need access to the collaboration engine.

### Adding as a Submodule

To add Engram v3 as a submodule to your project:

```bash
# Add the submodule to your repository
git submodule add https://github.com/nkkko/engram-v3.git engram-v3

# Initialize and update the submodule
git submodule update --init --recursive
```

### Updating the Submodule

To update the submodule to the latest version:

```bash
# Navigate to the submodule directory
cd engram-v3

# Pull the latest changes
git pull origin main

# Navigate back to the parent repository
cd ..

# Commit the updated submodule reference
git add engram-v3
git commit -m "Update Engram v3 submodule to latest version"
```

### Using in Your Project

After adding Engram v3 as a submodule, you can:

1. Import its Go packages in your code
2. Build and run it as a standalone service next to your application
3. Embed it directly in your application by importing the engine package

Example usage in a Go application:

```go
import (
    "context"
    "github.com/nkkko/engram-v3/internal/engine"
)

func main() {
    // Create default configuration
    config := engine.DefaultConfig()
    
    // Initialize the engine with all components
    eng, err := engine.CreateEngine(config)
    if err != nil {
        panic(err)
    }
    
    // Create context for the engine
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Start the engine
    if err := eng.Start(ctx); err != nil {
        panic(err)
    }
    
    // Your application code here
    
    // Graceful shutdown
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutdownCancel()
    if err := eng.Shutdown(shutdownCtx); err != nil {
        panic(err)
    }
}
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT

## Contact

Nikola - [github.com/nkkko](https://github.com/nkkko)