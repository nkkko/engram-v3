# Engram v3 Health Checks and Monitoring

This document describes the health check and monitoring capabilities of Engram v3.

## Health Check Endpoints

Engram v3 provides the following health check endpoints:

### Liveness Check

```
GET /healthz
```

The liveness check verifies that the server is running and responsive. It returns a simple `200 OK` response with the body `OK` if the server is alive.

**Example Response:**
```
HTTP/1.1 200 OK
Content-Type: text/plain
Content-Length: 2

OK
```

Use this endpoint for:
- Kubernetes liveness probes
- Load balancer health checks
- Basic availability monitoring

### Readiness Check

```
GET /readyz
```

The readiness check verifies that the server is ready to handle requests. It checks that all subsystems (storage, router, search, vector search, etc.) are properly initialized and ready to serve traffic.

**Example Response:**
```
HTTP/1.1 200 OK
Content-Type: text/plain
Content-Length: 2

OK
```

Use this endpoint for:
- Kubernetes readiness probes
- Load balancer backend readiness
- Service discovery systems

### Component Health Check

```
GET /healthz/components
```

This endpoint provides detailed health information for each component, including version information and status.

**Example Response:**
```json
{
  "status": "healthy",
  "components": {
    "api": {
      "status": "healthy",
      "version": "v3.2.1"
    },
    "storage": {
      "status": "healthy",
      "type": "badger",
      "version": "v3.0.0"
    },
    "router": {
      "status": "healthy",
      "active_subscriptions": 5
    },
    "search": {
      "status": "healthy",
      "backend": "badger",
      "indexed_units": 1250
    },
    "vector_search": {
      "status": "healthy",
      "backend": "weaviate",
      "connection": "ok"
    },
    "lock_manager": {
      "status": "healthy",
      "active_locks": 3
    }
  },
  "timestamp": "2023-09-23T15:42:01Z"
}
```

## Prometheus Metrics

Engram v3 exposes Prometheus-compatible metrics at:

```
GET /metrics
```

This endpoint returns metrics in the standard Prometheus text format.

### Available Metrics

#### System Metrics

- `engram_system_uptime_seconds` - Time since server startup
- `engram_system_memory_bytes{type="heap|stack|other"}` - Memory usage by category
- `engram_system_goroutines` - Number of active goroutines
- `engram_build_info{version, go_version, commit}` - Build information

#### API Metrics

- `engram_api_requests_total{method="GET|POST|PUT|DELETE", path="/path", status="200|400|500"}` - Count of API requests by method, path, and status
- `engram_api_request_duration_seconds{method="GET|POST|PUT|DELETE", path="/path"}` - Histogram of request durations by method and path
- `engram_api_request_size_bytes{method="GET|POST|PUT|DELETE", path="/path"}` - Histogram of request sizes
- `engram_api_response_size_bytes{method="GET|POST|PUT|DELETE", path="/path"}` - Histogram of response sizes

#### Storage Metrics

- `engram_storage_operations_total{operation="read|write|delete|list"}` - Count of storage operations by type
- `engram_storage_operation_duration_seconds{operation="read|write|delete|list"}` - Histogram of storage operation durations
- `engram_storage_batch_size{operation="read|write"}` - Histogram of batch sizes
- `engram_storage_errors_total{operation="read|write|delete|list", error_type="not_found|already_exists|internal"}` - Count of storage errors by type
- `engram_badger_compaction_total` - Count of BadgerDB compactions
- `engram_badger_gc_total` - Count of BadgerDB garbage collections
- `engram_badger_disk_usage_bytes{type="vlog|lsm"}` - Disk usage by BadgerDB

#### Work Unit Metrics

- `engram_work_units_total{type="1|2|3|4|5|6|7|8"}` - Total number of work units by type
- `engram_contexts_total` - Total number of contexts
- `engram_relationships_total{type="1|2|3|4|5|6|7"}` - Total number of relationships by type

#### Router Metrics

- `engram_router_subscriptions_total` - Current number of active subscriptions
- `engram_router_events_total{status="delivered|dropped"}` - Count of events processed by the router
- `engram_router_broadcast_duration_seconds` - Histogram of broadcast operation durations
- `engram_router_queue_size{subscription="context_id"}` - Gauge of queue size per subscription

#### Lock Manager Metrics

- `engram_locks_total{status="active|expired"}` - Count of locks by status
- `engram_lock_contentions_total` - Count of lock contention events
- `engram_lock_wait_duration_seconds` - Histogram of lock wait times
- `engram_lock_hold_duration_seconds` - Histogram of lock hold times
- `engram_lock_operations_total{operation="acquire|release|check|refresh"}` - Count of lock operations

#### Search Metrics

- `engram_search_queries_total{status="success|error", backend="badger"}` - Count of search queries
- `engram_search_query_duration_seconds{backend="badger"}` - Histogram of search query durations
- `engram_search_index_operations_total{operation="add|update|delete", backend="badger"}` - Count of index operations
- `engram_search_index_duration_seconds{operation="add|update|delete", backend="badger"}` - Histogram of index operation durations
- `engram_search_index_batch_size{backend="badger"}` - Histogram of index batch sizes

#### Vector Search Metrics

- `engram_vector_search_queries_total{status="success|error", backend="weaviate"}` - Count of vector search queries
- `engram_vector_search_query_duration_seconds{backend="weaviate"}` - Histogram of vector search query durations
- `engram_vector_index_operations_total{operation="add|update|delete", backend="weaviate"}` - Count of vector index operations
- `engram_vector_index_duration_seconds{operation="add|update|delete", backend="weaviate"}` - Histogram of vector index operation durations
- `engram_vector_index_batch_size{backend="weaviate"}` - Histogram of vector index batch sizes
- `engram_embedding_requests_total{status="success|error", model="model_name"}` - Count of embedding requests
- `engram_embedding_cache_hits_total` - Count of embedding cache hits
- `engram_embedding_duration_seconds{model="model_name"}` - Histogram of embedding operation durations

#### Connection Metrics

- `engram_websocket_connections_total` - Current number of active WebSocket connections
- `engram_sse_connections_total` - Current number of active SSE connections
- `engram_connection_duration_seconds{type="websocket|sse"}` - Histogram of connection durations
- `engram_connection_errors_total{type="websocket|sse", error="closed|timeout|protocol"}` - Count of connection errors

#### Telemetry Metrics

- `engram_trace_spans_total{status="ok|error"}` - Total number of trace spans created
- `engram_trace_export_errors_total` - Total number of trace export errors

## Integration with Monitoring Systems

### Prometheus Integration

To configure Prometheus to scrape Engram v3 metrics, add the following to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'engram'
    static_configs:
      - targets: ['engram:8080']
    metrics_path: /metrics
    scrape_interval: 15s
```

### Grafana Dashboard

A sample Grafana dashboard configuration is provided in the `./monitoring/grafana/` directory. Import this dashboard to visualize Engram v3 metrics.

Dashboard panels include:
- API Request Rates & Latencies
- Storage Operation Performance
- Work Unit Statistics
- Vector Search Performance
- Lock Contention Metrics
- BadgerDB Health & Performance
- Event Router Throughput
- WebSocket & SSE Connections

### Alert Examples

Example Prometheus alerts for Engram v3:

```yaml
groups:
- name: engram-alerts
  rules:
  - alert: EngramHighErrorRate
    expr: sum(rate(engram_api_requests_total{status=~"5.."}[5m])) / sum(rate(engram_api_requests_total[5m])) > 0.05
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "High API error rate"
      description: "Engram error rate is above 5% for 5 minutes ({{ $value | printf \"%.2f\" }}%)"

  - alert: EngramSlowRequests
    expr: histogram_quantile(0.95, sum(rate(engram_api_request_duration_seconds_bucket[5m])) by (le)) > 0.5
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Slow API requests"
      description: "95th percentile of request duration is above 500ms for 5 minutes ({{ $value | printf \"%.2f\" }}s)"

  - alert: EngramInstanceDown
    expr: up{job="engram"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Engram instance down"
      description: "Engram instance has been down for more than 1 minute"
      
  - alert: EngramBadgerDiskSpaceHigh
    expr: engram_badger_disk_usage_bytes / node_filesystem_avail_bytes{mountpoint="/app/data"} > 0.8
    for: 15m
    labels:
      severity: warning
    annotations:
      summary: "BadgerDB disk usage high"
      description: "BadgerDB is using more than 80% of available disk space for 15 minutes"
      
  - alert: EngramVectorSearchErrors
    expr: rate(engram_vector_search_queries_total{status="error"}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Vector search errors"
      description: "Vector search is experiencing errors at a rate > 0.1 per second for 5 minutes"
```

## OpenTelemetry Tracing

Engram v3 includes OpenTelemetry tracing to provide detailed insights into request flows and component interactions.

### Trace Points

Engram v3 creates traces for:

- API requests from start to finish
- Storage operations
- Search queries and indexing
- Vector search and embedding requests
- Lock acquisition and release
- Event routing and broadcasts

### Viewing Traces

Traces can be viewed in any OpenTelemetry-compatible trace viewer such as Jaeger, Zipkin, or cloud-based APM solutions.

Example with Jaeger:
1. Access the Jaeger UI: `http://localhost:16686`
2. Select "engram" from the service dropdown
3. Specify search criteria
4. View the trace timeline and details

### Trace Context Propagation

Engram v3 propagates trace context across:
- Internal component boundaries
- API requests and responses
- WebSocket and SSE connections
- Background operations

## Health Check Best Practices

### Docker Health Checks

Configure Docker health checks to ensure container health:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q --spider http://localhost:8080/healthz || exit 1
```

For Docker Compose:

```yaml
services:
  engram:
    # ...
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
```

### Kubernetes Probes

Configure Kubernetes probes for orchestrated deployments:

```yaml
spec:
  containers:
  - name: engram
    # ...
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080
      initialDelaySeconds: 10
      periodSeconds: 30
      timeoutSeconds: 5
      failureThreshold: 3
    readinessProbe:
      httpGet:
        path: /readyz
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 10
      timeoutSeconds: 3
      failureThreshold: 2
```

## Structured Logging

Engram v3 logs are structured in JSON format for easy integration with log aggregation systems:

```json
{
  "level": "info",
  "time": "2023-08-08T12:34:56Z",
  "component": "api",
  "message": "Request processed",
  "method": "GET",
  "path": "/contexts/123",
  "status": 200,
  "duration_ms": 12.5,
  "agent_id": "example-agent",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7"
}
```

Log levels:
- `debug`: Detailed debugging information
- `info`: Normal operational messages
- `warn`: Warning conditions that don't affect operation
- `error`: Error conditions that affect operation

Common logging patterns to monitor:
- ERROR level logs for critical issues
- Patterns like "connection error" or "database error"
- Unexpected spikes in request duration
- Authentication failures
- Vector search connection issues
- BadgerDB compaction failures

## Custom Monitoring Script

Example monitoring script that checks Engram v3 health and sends alerts:

```bash
#!/bin/bash

# Configuration
ENGRAM_URL="http://localhost:8080"
SLACK_WEBHOOK="https://hooks.slack.com/services/your/webhook/url"

# Check if Engram is healthy
health_status=$(curl -s -o /dev/null -w "%{http_code}" ${ENGRAM_URL}/healthz)

if [ "${health_status}" != "200" ]; then
  # Engram is not healthy, send alert
  curl -X POST -H 'Content-type: application/json' \
    --data "{\"text\":\"⚠️ Engram health check failed! Status: ${health_status}\"}" \
    ${SLACK_WEBHOOK}
  exit 1
fi

# Get component health
component_health=$(curl -s ${ENGRAM_URL}/healthz/components)
vector_search_status=$(echo "${component_health}" | jq -r '.components.vector_search.status')

if [ "${vector_search_status}" != "healthy" ]; then
  # Vector search is not healthy, send alert
  curl -X POST -H 'Content-type: application/json' \
    --data "{\"text\":\"⚠️ Engram vector search is not healthy! Status: ${vector_search_status}\"}" \
    ${SLACK_WEBHOOK}
fi

# Get metrics and check for issues
metrics=$(curl -s ${ENGRAM_URL}/metrics)

# Check for high error rate
error_rate=$(echo "${metrics}" | grep 'engram_api_requests_total.*status=\"5' | awk '{sum+=$2} END {print sum}')
total_requests=$(echo "${metrics}" | grep 'engram_api_requests_total' | awk '{sum+=$2} END {print sum}')

if [ "${total_requests}" != "0" ] && [ "${error_rate}" != "" ]; then
  error_percentage=$(echo "scale=2; ${error_rate} / ${total_requests} * 100" | bc)
  
  if (( $(echo "${error_percentage} > 5" | bc -l) )); then
    # Error rate is too high, send alert
    curl -X POST -H 'Content-type: application/json' \
      --data "{\"text\":\"⚠️ Engram error rate is high: ${error_percentage}%\"}" \
      ${SLACK_WEBHOOK}
  fi
fi

# Check BadgerDB disk usage
badger_disk_usage=$(echo "${metrics}" | grep 'engram_badger_disk_usage_bytes' | awk '{sum+=$2} END {print sum}')
if [ "${badger_disk_usage}" != "" ]; then
  # Convert to MB for readability
  disk_usage_mb=$(echo "scale=2; ${badger_disk_usage} / 1024 / 1024" | bc)
  
  if (( $(echo "${disk_usage_mb} > 1000" | bc -l) )); then
    # Disk usage is high, send alert
    curl -X POST -H 'Content-type: application/json' \
      --data "{\"text\":\"⚠️ Engram BadgerDB disk usage is high: ${disk_usage_mb} MB\"}" \
      ${SLACK_WEBHOOK}
  fi
fi

# Success
echo "Engram health check passed"
exit 0
```

## Troubleshooting Common Issues

### Failing Health Checks

If health checks are failing:

1. Check server logs: `docker logs engram`
2. Verify network connectivity: `curl -v http://localhost:8080/healthz`
3. Check resource usage: `docker stats engram`
4. Inspect container health: `docker inspect --format='{{json .State.Health}}' engram`
5. Check component health: `curl http://localhost:8080/healthz/components`

### High Error Rates

If you see high error rates in the metrics:

1. Check server logs for error patterns
2. Look at the specific API endpoints causing errors
3. Check client request patterns
4. Verify resource limits (memory, connections)
5. Check for BadgerDB errors in logs

### Vector Search Issues

If vector search is not working:

1. Verify Weaviate connectivity: `curl http://weaviate:8080/v1/.well-known/ready`
2. Check embedding service configuration and API keys
3. Look for "embedding" or "vector" errors in logs
4. Check vector search metrics for error patterns
5. Verify schema creation in Weaviate

### Performance Issues

For performance problems:

1. Check `engram_api_request_duration_seconds` metrics
2. Monitor BadgerDB metrics for compaction issues
3. Check vector search performance metrics
4. Monitor system resources (CPU, memory, disk I/O)
5. Check for lock contentions
6. Analyze client request patterns

### Storage Issues

For storage-related problems:

1. Check BadgerDB disk usage metrics
2. Look for I/O errors in logs
3. Verify storage volume permissions
4. Check for BadgerDB compaction or GC issues
5. Monitor LSM and value log size metrics

## Conclusion

Proper health monitoring is essential for running Engram v3 reliably in production. Utilize the health check endpoints, Prometheus metrics, OpenTelemetry tracing, and structured logging to ensure your deployment remains healthy and performant.