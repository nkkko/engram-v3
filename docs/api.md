# Engram v3 API Documentation

Engram v3 provides a comprehensive HTTP API for managing work units, contexts, and locks. This document describes the available endpoints, request formats, and response structures.

## Base URL

All API endpoints are relative to your Engram v3 server's base URL, for example:

```
http://localhost:8080
```

## Authentication

Engram v3 uses an agent-based authentication model. Each request must include an `X-Agent-ID` header with a valid agent ID.

```
X-Agent-ID: your-agent-id
```

## API Endpoints

### Health Checks

#### GET /healthz

Checks if the server is running.

**Response**:
- Status: `200 OK`
- Body: `OK`

#### GET /readyz

Checks if the server is ready to handle requests.

**Response**:
- Status: `200 OK`
- Body: `OK`

### Metrics

#### GET /metrics

Returns Prometheus-compatible metrics data.

**Response**:
- Status: `200 OK`
- Body: Prometheus metrics in text format

### Work Units

Work units are the core entities in Engram, representing atomic pieces of information such as messages, reasoning steps, tool calls, etc.

#### POST /workunit

Creates a new work unit.

**Request Body**:
```json
{
  "context_id": "string",
  "agent_id": "string",
  "type": number,
  "meta": {
    "key1": "value1",
    "key2": "value2"
  },
  "payload": "base64-encoded-data",
  "relationships": [
    {
      "target_id": "string",
      "type": number,
      "meta": {
        "key1": "value1"
      }
    }
  ]
}
```

Work unit types:
- `0`: `UNKNOWN`
- `1`: `MESSAGE` - Conversation messages
- `2`: `REASONING` - Agent reasoning steps
- `3`: `TOOL_CALL` - Tool invocation
- `4`: `TOOL_RESULT` - Tool output
- `5`: `FILE_CHANGE` - Code/document change
- `6`: `TASK_STATUS` - Task status update
- `7`: `ERROR` - Error information
- `8`: `SYSTEM` - System notifications

Relationship types:
- `0`: `UNSPECIFIED`
- `1`: `SEQUENTIAL` - Chronological order
- `2`: `CAUSES` - Causal relationship
- `3`: `DEPENDS_ON` - Dependency
- `4`: `REFERENCES` - Reference or citation
- `5`: `RESPONDS_TO` - Response to another unit
- `6`: `MODIFIES` - Modifies another unit
- `7`: `REPLACES` - Replaces another unit

**Response**:
- Status: `201 Created`
- Body:
```json
{
  "work_unit": {
    "id": "string",
    "context_id": "string",
    "agent_id": "string",
    "type": number,
    "ts": "timestamp",
    "meta": {
      "key1": "value1",
      "key2": "value2"
    },
    "payload": "base64-encoded-data",
    "relationships": [
      {
        "target_id": "string",
        "type": number,
        "meta": {
          "key1": "value1"
        }
      }
    ]
  }
}
```

#### GET /workunit/:id

Retrieves a work unit by ID.

**Parameters**:
- `id`: The ID of the work unit to retrieve

**Response**:
- Status: `200 OK`
- Body:
```json
{
  "work_unit": {
    "id": "string",
    "context_id": "string",
    "agent_id": "string",
    "type": number,
    "ts": "timestamp",
    "meta": {
      "key1": "value1",
      "key2": "value2"
    },
    "payload": "base64-encoded-data",
    "relationships": [
      {
        "target_id": "string",
        "type": number,
        "meta": {
          "key1": "value1"
        }
      }
    ]
  }
}
```

#### GET /workunits/:contextId

Lists work units for a context.

**Parameters**:
- `contextId`: The ID of the context to list work units for
- `limit`: (query) Maximum number of work units to return (default: 100, max: 1000)
- `cursor`: (query) Pagination cursor from previous response
- `reverse`: (query) Whether to return results in reverse chronological order (default: false)

**Response**:
- Status: `200 OK`
- Body:
```json
{
  "work_units": [
    {
      "id": "string",
      "context_id": "string",
      "agent_id": "string",
      "type": number,
      "ts": "timestamp",
      "meta": {
        "key1": "value1",
        "key2": "value2"
      },
      "payload": "base64-encoded-data",
      "relationships": [
        {
          "target_id": "string",
          "type": number,
          "meta": {
            "key1": "value1"
          }
        }
      ]
    }
  ],
  "next_cursor": "string"
}
```

### Contexts

Contexts represent logical groupings of work units and agents.

#### GET /contexts/:id

Retrieves a context by ID.

**Parameters**:
- `id`: The ID of the context to retrieve

**Response**:
- Status: `200 OK`
- Body:
```json
{
  "context": {
    "id": "string",
    "display_name": "string",
    "agent_ids": [
      "string"
    ],
    "pinned_units": [
      "string"
    ],
    "meta": {
      "key1": "value1",
      "key2": "value2"
    },
    "created_at": "timestamp",
    "updated_at": "timestamp"
  }
}
```

#### PATCH /contexts/:id

Updates a context. If the context doesn't exist, it will be created.

**Parameters**:
- `id`: The ID of the context to update

**Request Body**:
```json
{
  "display_name": "string",
  "add_agents": [
    "string"
  ],
  "remove_agents": [
    "string"
  ],
  "pin_units": [
    "string"
  ],
  "unpin_units": [
    "string"
  ],
  "meta": {
    "key1": "value1",
    "key2": "value2"
  }
}
```

**Response**:
- Status: `200 OK`
- Body:
```json
{
  "context": {
    "id": "string",
    "display_name": "string",
    "agent_ids": [
      "string"
    ],
    "pinned_units": [
      "string"
    ],
    "meta": {
      "key1": "value1",
      "key2": "value2"
    },
    "created_at": "timestamp",
    "updated_at": "timestamp"
  }
}
```

### Locks

Locks provide exclusive access to resources.

#### POST /locks

Acquires a lock on a resource.

**Request Body**:
```json
{
  "resource_path": "string",
  "agent_id": "string",
  "ttl_seconds": number
}
```

**Response**:
- Status: `200 OK`
- Body:
```json
{
  "lock": {
    "resource_path": "string",
    "holder_agent": "string",
    "expires_at": "timestamp",
    "lock_id": "string"
  }
}
```

#### GET /locks/:resource

Retrieves information about a lock.

**Parameters**:
- `resource`: The resource path to check

**Response**:
- Status: `200 OK`
- Body:
```json
{
  "lock": {
    "resource_path": "string",
    "holder_agent": "string",
    "expires_at": "timestamp",
    "lock_id": "string"
  }
}
```

#### DELETE /locks/:resource

Releases a lock on a resource.

**Parameters**:
- `resource`: The resource path to release
- `agent_id`: (query) The ID of the agent releasing the lock
- `lock_id`: (query) The ID of the lock to release

**Response**:
- Status: `200 OK`
- Body:
```json
{
  "released": true
}
```

### Search

#### POST /search

Searches for work units.

**Request Body**:
```json
{
  "query": "string",
  "context_id": "string",
  "agent_ids": [
    "string"
  ],
  "types": [
    "string"
  ],
  "meta_filters": {
    "key1": "value1",
    "key2": "value2"
  },
  "limit": number,
  "offset": number
}
```

**Response**:
- Status: `200 OK`
- Body:
```json
{
  "results": [
    {
      // Work unit object
    }
  ],
  "total_count": number,
  "limit": number,
  "offset": number
}
```

### Real-time Updates

#### WebSocket: /stream

Connects to the WebSocket endpoint for real-time updates.

**Query Parameters**:
- `context`: The context ID to subscribe to

**Headers**:
- `X-Agent-ID`: The ID of the agent connecting

The WebSocket connection will receive work unit events in real-time as they are created or updated. Messages are sent as JSON objects representing work units.

Example message:
```json
{
  "id": "string",
  "context_id": "string",
  "agent_id": "string",
  "type": number,
  "ts": "timestamp",
  "meta": {
    "key1": "value1"
  },
  "payload": "base64-encoded-data"
}
```

#### Server-Sent Events: /stream/sse

Connects to the Server-Sent Events endpoint for real-time updates.

**Query Parameters**:
- `context`: The context ID to subscribe to

**Headers**:
- `X-Agent-ID`: The ID of the agent connecting

The SSE connection will receive work unit events in real-time as they are created or updated. Events are sent with the following format:

```
event: workunit
data: {"id":"string","context_id":"string","agent_id":"string",...}

```

## Error Handling

All API endpoints return appropriate HTTP status codes:

- `200 OK`: Request succeeded
- `201 Created`: Resource created successfully
- `400 Bad Request`: Invalid request format or parameters
- `401 Unauthorized`: Authentication failure
- `403 Forbidden`: Permission denied
- `404 Not Found`: Resource not found
- `409 Conflict`: Resource conflict (e.g., lock already held)
- `500 Internal Server Error`: Server error

Error responses include a JSON object with an `error` field:

```json
{
  "error": "Error message"
}
```

## Rate Limiting

The API may apply rate limiting based on agent ID. If rate limited, the response will include:

- Status: `429 Too Many Requests`
- Headers:
  - `X-RateLimit-Limit`: Requests allowed per window
  - `X-RateLimit-Remaining`: Requests remaining in current window
  - `X-RateLimit-Reset`: Time (in seconds) when the rate limit resets

## Pagination

List endpoints return paginated results. To fetch additional pages:

1. Include the `next_cursor` value from the previous response as the `cursor` query parameter
2. If `next_cursor` is empty, there are no more results

## Examples

### Creating a Work Unit

```bash
curl -X POST http://localhost:8080/workunit \
  -H "Content-Type: application/json" \
  -H "X-Agent-ID: example-agent" \
  -d '{
    "context_id": "ctx-123",
    "type": 1,
    "payload": "SGVsbG8gd29ybGQ="
  }'
```

### Listing Work Units for a Context

```bash
curl -X GET "http://localhost:8080/workunits/ctx-123?limit=10" \
  -H "X-Agent-ID: example-agent"
```

### Acquiring a Lock

```bash
curl -X POST http://localhost:8080/locks \
  -H "Content-Type: application/json" \
  -H "X-Agent-ID: example-agent" \
  -d '{
    "resource_path": "/example/resource.txt",
    "ttl_seconds": 60
  }'
```

### Streaming Updates via WebSocket

```javascript
const ws = new WebSocket("ws://localhost:8080/stream?context=ctx-123");
ws.onmessage = (event) => {
  const workUnit = JSON.parse(event.data);
  console.log("Received work unit:", workUnit);
};
```