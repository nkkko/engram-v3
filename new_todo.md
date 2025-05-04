# Engram v3 Implementation Tasks

## Project Setup

## Core Components

## Implementation Details

## Testing & Validation

## Documentation

## Integration & Deployment

## Performance Optimization

## Storage Migration (Badger) - https://github.com/hypermodeinc/badger

## Test Fixes and Improvements
- [ ] Fix LockManager type mismatch - update MockStorage to match StorageInterface properly
- [ ] Replace SQLite-based search with Badger implementation for unified storage
- [ ] Remove unused grocksdb dependency from internal/storage/pool.go
- [ ] Implement full implementations instead of mocks for more reliable testing
- [ ] Update tests to work with real implementations instead of mocks
- [ ] Fix Dockerfile to properly support SQLite FTS5 if keeping SQLite
- [ ] Ensure consistent interface usage across all implementations

## Project Setup & Core Engine
- [ ] Implement core engine wiring in `cmd/engram/main.go` (Currently placeholder).
- [ ] Define and implement central configuration package (`internal/config`) (PLAN Phase 2).
- [ ] Replace `http.HandleFunc` with a proper router (e.g., chi, gorilla/mux) (PLAN Phase 2).
- [ ] Apply dependency injection for component wiring in `main.go` (PLAN Phase 2).

## Refactoring & Architecture (from PLAN.md)
- [ ] Phase 1: Integrate `golangci-lint`, pre-commit hooks, Makefile targets for lint/fmt.
- [ ] Phase 2: Define core component interfaces (`internal/domain` or similar).
- [ ] Phase 3: Refactor API handlers, adopt `context.Context`, standardize errors, generate OpenAPI docs.
- [ ] Phase 4: Abstract storage implementation behind `StorageEngine` interface, implement factory.
- [ ] Phase 5: Abstract event routing behind `EventRouter` interface.
- [ ] Phase 6: Enhance Lock Manager robustness, TTL handling, metrics, context cancellation.
- [ ] Phase 7: Define `SearchEngine` interface, implement pluggable backends.
- [ ] Phase 8: Integrate OpenTelemetry, structured logging (zap/zerolog), expand Prometheus metrics.
- [ ] Phase 10: Consolidate/update documentation, add CONTRIBUTING.md, add client examples.

## Storage & Persistence
- [ ] Optimize BadgerDB batching and WAL configuration (PLAN Phase 4).
- [ ] Remove unused RocksDB dependency from `internal/storage/pool.go` (if still present).

## Search & Retrieval
- [ ] Implement efficient relationship querying based on `proto.Relationship`.
- [ ] Implement `SearchEngine` interface (PLAN Phase 7).
- [ ] Integrate a Vector Search backend using https://github.com/weaviate/weaviate
- [ ] Benchmark search query performance and optimize (PLAN Phase 7).

## LLM Context Optimization Features
- [ ] Design & Implement API(s) for LLM-optimized context retrieval (consider recency, type, relationships, semantic relevance).
- [ ] Add specific query capabilities for retrieving units based on `Relationship` types.
- [ ] Explore adding optional summarization capabilities or hooks (potentially via external LLM call).
- [ ] Ensure efficient retrieval of `pinned_units` for context building.

## Real-time & Concurrency
- [ ] Rigorously benchmark stream fan-out latency under load (< 5 ms target).
- [ ] Add unit tests for `EventRouter` edge cases (slow subscribers, backpressure) (PLAN Phase 5).
- [ ] Ensure correct concurrency control across WAL, DB, and in-memory components.

## Testing & Reliability
- [ ] Increase unit test coverage across all packages (> 80% target) (PLAN Phase 9).
- [ ] Develop comprehensive integration tests (full engine spin-up) (PLAN Phase 9).
- [ ] Add contract tests against API definitions (OpenAPI/protobuf) (PLAN Phase 9).
- [ ] Implement chaos tests (WAL corruption, network partitions, etc.) (PLAN Phase 9).
- [ ] Implement benchmarks for core latency targets (WorkUnit write < 1ms, Query < 2ms) (PLAN Phase 4, 8).
- [ ] Fix LockManager type mismatch in tests - update MockStorage or test setup.
- [ ] Update tests to use real implementations where feasible instead of mocks.

## Operational Concerns
- [ ] Implement standardized structured logging (PLAN Phase 8).
- [ ] Integrate OpenTelemetry tracing (PLAN Phase 8).
- [ ] Expand Prometheus metrics as defined in `PLAN.md` (PLAN Phase 8).
- [ ] Implement robust error handling and reporting across components.
- [ ] Document scalability strategy (even if single-node first).

## Documentation
- [ ] Consolidate and update all documentation in `/docs` (PLAN Phase 10).
- [ ] Update architecture diagrams to reflect current/target state (PLAN Phase 10).
- [ ] Create `CONTRIBUTING.md` (PLAN Phase 10).
- [ ] Provide example clients (Go, Python, JS) (PLAN Phase 10).

# DONE
- [x] Create main directory structure
- [x] Create SPECS.md with technical specification
- [x] Set up README.md with documentation
- [x] Initialize Go module (go.mod)
- [x] Create Dockerfile for containerization
- [x] Add .gitignore and .dockerignore
- [x] Create Makefile for common tasks
- [x] Set up configuration example
- [x] Define Protocol Buffer schema (proto/engram.proto)
- [x] Implement engine coordinator (internal/engine/engine.go)
- [x] Implement storage engine (internal/storage/storage.go)
- [x] Implement event router (internal/router/router.go)
- [x] Implement notification hub (internal/notifier/notifier.go)
- [x] Implement lock manager (internal/lockmanager/lockmanager.go)
- [x] Implement search indexing (internal/search/search.go)
- [x] Implement HTTP/WebSocket API (internal/api/api.go)
- [x] Create client library (pkg/client/client.go)
- [x] WAL (Write-Ahead Log) for durability
- [x] Badger integration for persistent storage
- [x] WebSocket/SSE support for real-time updates
- [x] Lock acquisition and release mechanism
- [x] Context management functionality
- [x] Work unit creation and retrieval
- [x] Search functionality with SQLite FTS5
- [x] Implement basic storage tests
- [x] Implement simple validation tests
- [x] Run the validation tests successfully
- [x] Implement router tests
- [x] Implement lock manager tests
- [x] Implement API tests
- [x] Implement end-to-end tests
- [x] Perform benchmark tests (1ms target for work unit creation)
- [x] Validate WebSocket streaming with multiple clients
- [x] Test lock contention handling
- [x] Implement search tests
- [x] Implement metrics tests
- [x] Implement concurrency tests
- [x] Add benchmarks for optimized components
- [x] Add installation instructions
- [x] Document API surface
- [x] Add usage examples
- [x] Document configuration options
- [x] Add detailed API documentation
- [x] Add architecture diagrams
- [x] Create example client application
- [x] Test Docker build and run
- [x] Create deployment guide
- [x] Add health check documentation
- [x] Optimize WAL batching
- [x] Add instrumentation for metrics
- [x] Implement connection pooling
- [x] Add caching layer for hot paths
- [x] Optimize zero-copy broadcast buffers
- [x] Research Badger API and determine column family alternative approach (using prefixed keys)
- [x] Create new storage adapter implementation that uses Badger
- [x] Implement internal prefix scheme to replace column families
- [x] Remove connection pool (not needed with Badger transactions)
- [x] Update related engine code to use interface instead of concrete type
- [x] Modify Dockerfile to remove RocksDB dependencies
- [x] Update storage tests
- [x] Implement benchmark tests
- [x] Update documentation with new storage engine details
