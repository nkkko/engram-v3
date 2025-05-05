# Engram v3 Architecture (Updated)

This document describes the updated architecture of Engram v3, a distributed event-based storage and communication system designed for AI agents.

## System Overview

Engram v3 is built as a modular, event-driven system with the following key components:

```
┌─────────────────────────────────────────────────────────────┐
│                       Engram v3 Server                       │
│                                                             │
│  ┌─────────┐     ┌────────┐     ┌──────────────┐            │
│  │   API   │────>│ Engine │<───>│ Lock Manager │            │
│  └─────────┘     └────────┘     └──────────────┘            │
│      │               │                  │                    │
│      │               │                  │                    │
│      ▼               ▼                  ▼                    │
│  ┌─────────┐     ┌────────┐     ┌──────────────┐            │
│  │ Router  │<───>│ Storage │<───>│    Search    │            │
│  └─────────┘     └────────┘     └──────────────┘            │
│      │               │               │     │                 │
│      │               │               │     │                 │
│      ▼               ▼               ▼     ▼                 │
│  ┌─────────┐     ┌────────┐     ┌──────┐ ┌─────────┐        │
│  │Notifier │     │  WAL   │     │Badger│ │Weaviate │        │
│  └─────────┘     └────────┘     └──────┘ └─────────┘        │
│      │               │               │       │              │
│      │               ▼               ▼       │              │
│      │           ┌────────┐     ┌──────────┐ │              │
│      └──────────>│ Badger │<────┤  Vector  │<┘              │
│                  └────────┘     │ Embeddings│                │
│                                 └──────────┘                │
└─────────────────────────────────────────────────────────────┘
           │                               │
           ▼                               ▼
┌─────────────────────┐        ┌─────────────────────┐
│ WebSocket Clients   │        │     HTTP Clients     │
└─────────────────────┘        └─────────────────────┘
```

## Core Components

### Engine

The Engine serves as the central coordinator, initializing and managing the lifecycle of all other components. It handles startup, shutdown, and resource management, ensuring proper sequencing during these operations.

Key responsibilities:
- Component lifecycle management
- Graceful shutdown handling
- Resource allocation and cleanup
- System-wide configuration
- Telemetry and observability setup

### API

The API component provides HTTP and WebSocket endpoints for client interactions. It uses the Chi router for flexible and expressive routing with standardized error handling.

Key features:
- RESTful API for work unit and context operations
- WebSocket support for real-time updates
- Server-Sent Events (SSE) for browsers without WebSocket support
- Health check endpoints for monitoring
- Prometheus metrics endpoint
- OpenTelemetry integration for tracing

### Router

The Router manages subscriptions and directs events to interested clients based on their context subscriptions. It efficiently broadcasts work units to all relevant subscribers while minimizing redundant message delivery.

Key capabilities:
- Context-based subscription management
- Efficient event broadcasting
- Subscription cleanup for idle connections
- Fan-out message delivery
- Metrics for monitoring event flow

### Storage

The Storage component is responsible for persisting work units, contexts, and locks. It uses BadgerDB for high-performance key-value storage and includes a Write-Ahead Log (WAL) for durability.

Important features:
- Optimized BadgerDB configuration for performance
- Atomic operations with durability guarantees
- WAL batching for performance optimization
- Efficient querying and retrieval
- Pluggable storage backends (basic and optimized)

### Lock Manager

The Lock Manager provides distributed locking capabilities for resource coordination. It manages lock acquisition, release, and automatic expiration with enhanced features like retries and metrics.

Key aspects:
- Resource path-based lock identifiers
- TTL-based expiration
- Deadlock prevention through lexical ordering
- Context cancellation support
- Retry mechanism with exponential backoff
- Comprehensive metrics

### Search

The Search component enables both full-text search and vector semantic search capabilities. It supports multiple backends with a unified interface.

Notable features:
- Full-text indexing of work unit contents
- Vector search for semantic similarity using Weaviate
- Support for filtering by context, agent, and metadata
- Pluggable backends (BadgerDB and Weaviate)
- Relationship-based querying
- Batched indexing for performance

### Notifier

The Notifier manages real-time client connections and delivers events to connected clients. It supports both WebSocket and Server-Sent Events protocols.

Key functions:
- Connection lifecycle management
- Client authentication
- Heartbeat mechanism
- Efficient message delivery
- Broadcasting for multi-client updates

## Data Flow

1. **Client Request Flow**:
   - Client sends a request to the API
   - API validates the request and forwards it to the appropriate component
   - Component processes the request and returns a response
   - API sends the response back to the client

2. **Work Unit Creation Flow**:
   - Client sends a work unit creation request
   - API validates the request
   - Storage creates and persists the work unit
   - Router broadcasts the work unit to subscribed clients
   - Notifier delivers the work unit to connected clients
   - Search indexes the work unit (asynchronously)

3. **Subscription Flow**:
   - Client connects to the WebSocket or SSE endpoint
   - API authenticates the client
   - Router creates a subscription for the client
   - Notifier manages the client connection
   - Work units are delivered to the client in real-time

4. **Search Flow**:
   - Client sends a search request (text or semantic)
   - API validates the request
   - Search engine selects appropriate backend (Badger or Weaviate)
   - Backend executes the search query
   - Results are returned to the client

## Storage Architecture

Engram v3 uses a layered storage approach:

1. **In-Memory Layer**:
   - Recent and frequently accessed data
   - Lock information
   - Subscription state
   - LRU caches for performance

2. **BadgerDB Layer**:
   - Persistent storage for all data
   - Prefix-based organization for different data types
   - LSM-tree for efficient writes and reads
   - Optimized configuration for performance

3. **Write-Ahead Log (WAL)**:
   - Durability for all write operations
   - Batched writes for performance
   - Recovery mechanism for crashes
   - Automatic rotation based on size

4. **Search Index Layer**:
   - BadgerDB for basic text search
   - Weaviate for vector/semantic search
   - Asynchronous indexing
   - Query optimization

## Data Models

### Work Unit

The core entity for storing atomic information:

```
WorkUnit {
  id: string (UUID)
  context_id: string
  agent_id: string
  type: enum (MESSAGE, REASONING, TOOL_CALL, etc.)
  timestamp: datetime
  metadata: map<string, string>
  payload: bytes
  relationships: array<Relationship>
}
```

### Context

A logical grouping of work units and agents:

```
Context {
  id: string
  display_name: string
  agent_ids: array<string>
  pinned_units: array<string>
  metadata: map<string, string>
  created_at: datetime
  updated_at: datetime
}
```

### Lock

Exclusive access control for resources:

```
Lock {
  resource_path: string
  holder_agent: string
  expires_at: datetime
  lock_id: string
}
```

### Relationship

Connections between work units:

```
Relationship {
  target_id: string
  type: enum (SEQUENTIAL, CAUSES, DEPENDS_ON, etc.)
  metadata: map<string, string>
}
```

## Observability Architecture

Engram v3 includes comprehensive observability features:

### Logging

- Structured JSON logging with zerolog
- Context-aware logging with trace correlation
- Configurable log levels (debug, info, warn, error)
- Console or JSON output formats

### Metrics

- Prometheus-compatible metrics
- Detailed metrics for all components
- Histograms for latency measurements
- Counters for operations and errors
- Gauges for resource utilization

### Tracing

- OpenTelemetry integration
- Distributed tracing across components
- Context propagation
- Span attributes for detailed analysis
- Trace sampling configuration

## Deployment Architecture

Engram v3 is designed to be deployed in various environments:

### Standalone Deployment

```
┌─────────────────┐
│  Engram Server  │
└─────────────────┘
        │
        ▼
┌─────────────────┐
│  Local Storage  │
└─────────────────┘
```

### Containerized Deployment

```
┌───────────────┐
│   K8s/Docker  │
│               │
│ ┌───────────┐ │
│ │   Engram  │ │
│ └───────────┘ │
│       │       │
│       ▼       │
│ ┌───────────┐ │
│ │  Volumes  │ │
│ └───────────┘ │
└───────────────┘
```

### High-Availability Deployment

```
┌──────────────────┐   ┌──────────────────┐
│  Engram Server 1  │   │  Engram Server 2  │
└──────────────────┘   └──────────────────┘
         │                      │
         └──────────┬──────────┘
                    │
         ┌──────────▼──────────┐
         │  Shared Storage     │
         └─────────────────────┘
```

## Performance Considerations

Engram v3 is optimized for:

1. **Low Latency**: Sub-millisecond work unit creation target
2. **High Throughput**: Efficient batching and concurrency
3. **Real-Time Updates**: Minimal delay for event delivery
4. **Resource Efficiency**: Optimized memory and CPU usage
5. **Scalability**: Designed to handle large numbers of work units and clients

## Security Architecture

Engram v3 security is designed with multiple layers:

1. **Authentication**: Agent-based authentication with configurable providers
2. **Authorization**: Resource-based access control
3. **TLS**: Encrypted connections for all client communication
4. **Rate Limiting**: Protection against abuse
5. **Input Validation**: Strict validation of all client requests
6. **Error Handling**: Standardized error responses that don't leak sensitive information

## Future Enhancements

1. **Distributed Consensus**: For multi-node deployments
2. **Sharding**: For horizontal scaling
3. **Advanced Caching**: For improved read performance
4. **Event Sourcing**: For enhanced auditability
5. **Zero-Copy Buffers**: For maximum performance
6. **Federated Search**: For distributed search capabilities