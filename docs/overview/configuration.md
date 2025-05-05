# Configuration

Engram v3 is configured using a YAML file, typically named `config.yaml`, located in the application's root directory. Below is a breakdown of the available configuration options based on the example file.

## Server (`server`)

HTTP server settings.

*   `addr`: HTTP server address (e.g., `:8080`).
*   `max_body_size`: Maximum request body size in bytes (e.g., `1048576` for 1MB).
*   `read_timeout`: Read timeout in seconds (e.g., `5`).
*   `write_timeout`: Write timeout in seconds (e.g., `10`).
*   `idle_timeout`: Idle timeout in seconds (e.g., `120`).

## Storage (`storage`)

Data persistence and storage settings.

*   `data_dir`: Directory to store data files (e.g., `./data`).
*   `wal_sync_interval_ms`: Write-Ahead Log sync interval in milliseconds (e.g., `2`).
*   `wal_batch_size`: Number of work units per WAL batch (e.g., `64`).
*   `max_wal_size_mb`: Maximum WAL file size before rotation in megabytes (e.g., `512`).

## Router (`router`)

Subscription routing and buffering settings.

*   `max_buffer_size`: Maximum buffer size for subscription channels (e.g., `100`).
*   `cleanup_interval`: Interval in seconds for cleaning up resources (e.g., `30`).

## Notifier (`notifier`)

Client connection and notification settings.

*   `max_idle_time`: Maximum idle time in seconds before dropping a connection (e.g., `30`).
*   `heartbeat_interval`: Heartbeat interval in seconds (e.g., `5`).
*   `max_connections`: Maximum concurrent connections allowed (e.g., `10000`).

## Lock Manager (`locks`)

Distributed locking settings.

*   `default_ttl`: Default lock Time-To-Live in seconds (e.g., `60`).
*   `cleanup_interval`: Interval in seconds for cleaning up expired locks (e.g., `10`).
*   `max_locks_per_agent`: Maximum number of locks an agent can hold (e.g., `100`).

## Search (`search`)

Search indexing and querying settings.

*   `engine`: Search engine type (`sqlite` or `qdrant`).
*   `update_interval_ms`: Index update interval in milliseconds (e.g., `200`).
*   `max_batch_size`: Maximum batch size for index updates (e.g., `100`).
*   `default_limit`: Default number of search results to return (e.g., `100`).
*   `max_limit`: Maximum number of search results to return (e.g., `1000`).

## Authentication (`auth`)

Authentication and authorization settings.

*   `enabled`: Enable or disable authentication (`true`/`false`).
*   `jwt_secret`: Secret key for signing JWT tokens (Change this in production!).
*   `jwt_expiration_minutes`: JWT token expiration time in minutes (e.g., `60`).
*   `allow_anonymous`: Allow access without authentication (`true`/`false`).

## Logging (`logging`)

Logging configuration.

*   `level`: Log level (`debug`, `info`, `warn`, `error`).
*   `format`: Log format (`json`, `console`).
*   `tracing`: Enable OpenTelemetry tracing (`true`/`false`).
*   `otel_endpoint`: OpenTelemetry collector endpoint (e.g., `localhost:4317`).

## Metrics (`metrics`)

Prometheus metrics settings.

*   `enabled`: Enable or disable Prometheus metrics (`true`/`false`).
*   `endpoint`: Path for the metrics endpoint (e.g., `/metrics`).
*   `interval`: Metrics collection interval in seconds (e.g., `15`).

## Rate Limiting (`rate_limit`)

API rate limiting settings.

*   `enabled`: Enable or disable rate limiting (`true`/`false`).
*   `rpm`: Maximum requests per minute (e.g., `600`).
*   `rps`: Maximum requests per second (e.g., `10`).
*   `ws_messages_per_second`: Maximum WebSocket messages per second (e.g., `20`).