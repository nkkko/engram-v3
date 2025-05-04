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

The readiness check verifies that the server is ready to handle requests. It checks that all subsystems (storage, router, search, etc.) are properly initialized and ready to serve traffic.

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

## Prometheus Metrics

Engram v3 exposes Prometheus-compatible metrics at:

```
GET /metrics
```

This endpoint returns metrics in the standard Prometheus text format.

### Available Metrics

#### System Metrics

- `engram_system_uptime_seconds` - Time since server startup
- `engram_system_memory_bytes` - Memory usage by category
- `engram_system_goroutines` - Number of active goroutines

#### API Metrics

- `engram_api_requests_total{method="GET|POST|PUT|DELETE", path="/path", status="200|400|500"}` - Count of API requests by method, path, and status
- `engram_api_request_duration_seconds{method="GET|POST|PUT|DELETE", path="/path"}` - Histogram of request durations by method and path

#### Storage Metrics

- `engram_storage_operations_total{operation="read|write|delete"}` - Count of storage operations by type
- `engram_storage_operation_duration_seconds{operation="read|write|delete"}` - Histogram of storage operation durations
- `engram_work_units_total` - Total number of work units
- `engram_contexts_total` - Total number of contexts

#### Router Metrics

- `engram_router_subscriptions_total` - Current number of active subscriptions
- `engram_router_events_total{status="delivered|dropped"}` - Count of events processed by the router

#### Lock Manager Metrics

- `engram_locks_total{status="active|expired"}` - Count of locks by status
- `engram_lock_contentions_total` - Count of lock contention events

#### Connection Metrics

- `engram_websocket_connections_total` - Current number of active WebSocket connections
- `engram_sse_connections_total` - Current number of active SSE connections

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
```

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

## Log Monitoring

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
  "agent_id": "example-agent"
}
```

Common logging patterns to monitor:

- ERROR level logs for critical issues
- Patterns like "connection error" or "database error"
- Unexpected spikes in request duration
- Authentication failures

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

### High Error Rates

If you see high error rates in the metrics:

1. Check server logs for error patterns
2. Look at the specific API endpoints causing errors
3. Check client request patterns
4. Verify resource limits (memory, connections)

### Performance Issues

For performance problems:

1. Check `engram_api_request_duration_seconds` metrics
2. Monitor system resources (CPU, memory, disk I/O)
3. Check for lock contentions
4. Analyze client request patterns

## Conclusion

Proper health monitoring is essential for running Engram v3 reliably in production. Utilize the health check endpoints, Prometheus metrics, and logging to ensure your deployment remains healthy and performant.