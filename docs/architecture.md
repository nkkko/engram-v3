# Engram v3 Architecture

This document describes the architecture of Engram v3, a distributed event-based storage and communication system designed for AI agents.

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
│      │               │                  │                    │
│      │               │                  │                    │
│      ▼               ▼                  ▼                    │
│  ┌─────────┐     ┌────────┐     ┌──────────────┐            │
│  │Notifier │     │  WAL   │     │  SQLite FTS  │            │
│  └─────────┘     └────────┘     └──────────────┘            │
│      │               │                                       │
│      │               ▼                                       │
│      │           ┌────────┐                                  │
│      └──────────>│ RocksDB │                                 │
│                  └────────┘                                  │
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

### API

The API component provides HTTP and WebSocket endpoints for client interactions. It uses the Fiber web framework to deliver high-performance request handling with minimal overhead.

Key features:
- RESTful API for work unit and context operations
- WebSocket support for real-time updates
- Server-Sent Events (SSE) for browsers without WebSocket support
- Health check endpoints for monitoring
- Prometheus metrics endpoint

### Router

The Router manages subscriptions and directs events to interested clients based on their context subscriptions. It efficiently broadcasts work units to all relevant subscribers while minimizing redundant message delivery.

Key capabilities:
- Context-based subscription management
- Efficient event broadcasting
- Subscription cleanup for idle connections
- Fan-out message delivery

### Storage

The Storage component is responsible for persisting work units, contexts, and locks. It uses RocksDB for high-performance key-value storage and includes a Write-Ahead Log (WAL) for durability.

Important features:
- Column-family organization for different data types
- Atomic operations with durability guarantees
- WAL batching for performance optimization
- Efficient querying and retrieval

### Lock Manager

The Lock Manager provides distributed locking capabilities for resource coordination. It manages lock acquisition, release, and automatic expiration.

Key aspects:
- Resource path-based lock identifiers
- TTL-based expiration
- Deadlock prevention through lexical ordering
- Atomic lock operations

### Search

The Search component enables full-text search capabilities using SQLite with FTS5 (Full-Text Search) extension. It maintains an index of work units for efficient querying.

Notable features:
- Full-text indexing of work unit contents
- Support for filtering by context, agent, and metadata
- Asynchronous indexing for performance
- Query optimization

### Notifier

The Notifier manages real-time client connections and delivers events to connected clients. It supports both WebSocket and Server-Sent Events protocols.

Key functions:
- Connection lifecycle management
- Client authentication
- Heartbeat mechanism
- Efficient message delivery

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

## Storage Architecture

Engram v3 uses a layered storage approach:

1. **In-Memory Layer**:
   - Recent and frequently accessed data
   - Lock information
   - Subscription state

2. **RocksDB Layer**:
   - Persistent storage for all data
   - Column families for different data types
   - LSM-tree for efficient writes and reads

3. **Write-Ahead Log (WAL)**:
   - Durability for all write operations
   - Batched writes for performance
   - Recovery mechanism for crashes

4. **Search Index Layer**:
   - SQLite FTS5 for full-text search
   - Asynchronous indexing
   - Query optimization

## Data Models

### Work Unit

The core entity for storing atomic information:

```
WorkUnit {
  id: string (UUIDv7)
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

## Security Architecture

Engram v3 security is designed with multiple layers:

1. **Authentication**: Agent-based authentication with configurable providers
2. **Authorization**: Resource-based access control
3. **TLS**: Encrypted connections for all client communication
4. **Rate Limiting**: Protection against abuse
5. **Input Validation**: Strict validation of all client requests

## Performance Considerations

Engram v3 is optimized for:

1. **Low Latency**: Sub-millisecond work unit creation target
2. **High Throughput**: Efficient batching and concurrency
3. **Real-Time Updates**: Minimal delay for event delivery
4. **Resource Efficiency**: Optimized memory and CPU usage
5. **Scalability**: Designed to handle large numbers of work units and clients

## Future Enhancements

1. **Distributed Consensus**: For multi-node deployments
2. **Sharding**: For horizontal scaling
3. **Advanced Caching**: For improved read performance
4. **Event Sourcing**: For enhanced auditability
5. **Zero-Copy Buffers**: For maximum performance