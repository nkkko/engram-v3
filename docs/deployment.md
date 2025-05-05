# Engram v3 Deployment Guide

This guide covers how to deploy Engram v3 in various environments, with a focus on Docker-based deployments.

## Prerequisites

- Docker (20.10.0 or later)
- Docker Compose (optional, for multi-container setups)
- 2GB RAM minimum (4GB+ recommended)
- 1GB disk space minimum

## Docker Deployment

### Building the Docker Image

The repository includes a Dockerfile that creates an optimized, multi-stage build:

```bash
# Clone the repository (if you haven't already)
git clone https://github.com/nkkko/engram-v3.git
cd engram-v3

# Build the Docker image
docker build -t engram:v3 .
```

### Running the Container

Run Engram v3 with the default configuration:

```bash
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  engram:v3
```

This creates a container with:
- The HTTP API exposed on port 8080
- A persistent Docker volume for data storage

### Configuration

Engram v3 can be configured using environment variables or command-line arguments:

```bash
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  -e LOG_LEVEL=debug \
  -e MAX_CONNECTIONS=1000 \
  -e VECTOR_SEARCH_ENABLED=true \
  -e WEAVIATE_URL=http://weaviate:8080 \
  engram:v3 \
  --addr=:8080 \
  --data-dir=/app/data
```

Common configuration options:

| Option | Environment Variable | Default | Description |
|--------|---------------------|---------|-------------|
| --addr | ENGRAM_ADDR | :8080 | HTTP server address |
| --data-dir | DATA_DIR | ./data | Data storage location |
| --log-level | LOG_LEVEL | info | Logging level (debug, info, warn, error) |
| --max-connections | MAX_CONNECTIONS | 1000 | Maximum concurrent connections |
| --vector-search | VECTOR_SEARCH_ENABLED | false | Enable vector search capabilities |
| --weaviate-url | WEAVIATE_URL | http://localhost:8080 | Weaviate server URL for vector search |
| --weaviate-api-key | WEAVIATE_API_KEY | | API key for Weaviate (if required) |
| --embedding-model | EMBEDDING_MODEL | openai/text-embedding-ada-002 | Model to use for vector embeddings |
| --embedding-url | EMBEDDING_URL | https://api.openai.com/v1 | URL for embedding service |
| --embedding-key | EMBEDDING_KEY | | API key for embedding service |
| --telemetry-enabled | TELEMETRY_ENABLED | true | Enable OpenTelemetry tracing |
| --storage-optimized | STORAGE_OPTIMIZED | false | Use optimized storage configuration |

### Docker Compose Setup

For a more complete deployment with vector search, use Docker Compose:

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  engram:
    image: engram:v3
    container_name: engram
    ports:
      - "8080:8080"
    volumes:
      - engram-data:/app/data
    environment:
      - LOG_LEVEL=info
      - VECTOR_SEARCH_ENABLED=true
      - WEAVIATE_URL=http://weaviate:8080
      - TELEMETRY_ENABLED=true
      - STORAGE_OPTIMIZED=true
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    depends_on:
      - weaviate

  # Weaviate vector database for semantic search
  weaviate:
    image: semitechnologies/weaviate:1.19.6
    container_name: weaviate
    ports:
      - "9090:8080"
    environment:
      - QUERY_DEFAULTS_LIMIT=20
      - AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED=true
      - PERSISTENCE_DATA_PATH=/var/lib/weaviate
      - DEFAULT_VECTORIZER_MODULE=none
      - ENABLE_MODULES=text2vec-openai
    volumes:
      - weaviate-data:/var/lib/weaviate
    restart: unless-stopped

  # Prometheus for metrics
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    restart: unless-stopped
    depends_on:
      - engram

  # Jaeger for distributed tracing
  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: jaeger
    ports:
      - "16686:16686"  # Web UI
      - "4317:4317"    # OTLP gRPC
      - "4318:4318"    # OTLP HTTP
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    restart: unless-stopped

volumes:
  engram-data:
  weaviate-data:
  prometheus-data:
```

Create a `prometheus.yml` file for monitoring:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'engram'
    static_configs:
      - targets: ['engram:8080']
    metrics_path: /metrics
```

Start the services:

```bash
docker-compose up -d
```

## Health Monitoring

Engram v3 provides health check endpoints for monitoring:

- **Liveness Check**: `GET /healthz` - Returns HTTP 200 if the server is running
- **Readiness Check**: `GET /readyz` - Returns HTTP 200 if the server is ready to handle requests
- **Metrics**: `GET /metrics` - Returns Prometheus-compatible metrics

### Docker Health Checks

The Docker Compose example includes a health check that verifies the `/healthz` endpoint. You can also add this to a standalone Docker run command:

```bash
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  --health-cmd="wget -q --spider http://localhost:8080/healthz || exit 1" \
  --health-interval=30s \
  --health-timeout=5s \
  --health-retries=3 \
  --health-start-period=10s \
  engram:v3
```

### Prometheus Integration

Engram v3 exposes metrics in Prometheus format at the `/metrics` endpoint. The Docker Compose setup above includes a Prometheus server configured to scrape these metrics.

Key metrics:
- `engram_work_units_total` - Total number of work units created
- `engram_api_requests_total` - Total number of API requests
- `engram_api_request_duration_seconds` - API request duration histogram
- `engram_connection_count` - Current number of active connections
- `engram_storage_operations_total` - Total number of storage operations
- `engram_search_queries_total` - Total number of search queries
- `engram_vector_search_queries_total` - Total number of vector search queries
- `engram_search_query_duration_seconds` - Search query duration histogram

### OpenTelemetry Tracing

Engram v3 includes OpenTelemetry tracing for distributed observability. Traces are exported to the configured collector (Jaeger in the Docker Compose example).

Key trace points:
- API request handling
- Storage operations
- Search queries
- Lock operations
- Router event processing

## Kubernetes Deployment

For production environments, Kubernetes is recommended. Here's a basic deployment:

Create a file named `engram-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: engram
  labels:
    app: engram
spec:
  replicas: 1
  selector:
    matchLabels:
      app: engram
  template:
    metadata:
      labels:
        app: engram
    spec:
      containers:
      - name: engram
        image: engram:v3
        ports:
        - containerPort: 8080
        env:
        - name: LOG_LEVEL
          value: "info"
        - name: STORAGE_OPTIMIZED
          value: "true"
        - name: VECTOR_SEARCH_ENABLED
          value: "true"
        - name: WEAVIATE_URL
          value: "http://weaviate-service:8080"
        - name: TELEMETRY_ENABLED
          value: "true"
        volumeMounts:
        - name: data
          mountPath: /app/data
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: engram-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: engram
spec:
  selector:
    app: engram
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: engram-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

For Weaviate vector database, create a file named `weaviate-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: weaviate
  labels:
    app: weaviate
spec:
  replicas: 1
  selector:
    matchLabels:
      app: weaviate
  template:
    metadata:
      labels:
        app: weaviate
    spec:
      containers:
      - name: weaviate
        image: semitechnologies/weaviate:1.19.6
        ports:
        - containerPort: 8080
        env:
        - name: QUERY_DEFAULTS_LIMIT
          value: "20"
        - name: AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED
          value: "true"
        - name: PERSISTENCE_DATA_PATH
          value: "/var/lib/weaviate"
        - name: DEFAULT_VECTORIZER_MODULE
          value: "none"
        - name: ENABLE_MODULES
          value: "text2vec-openai"
        volumeMounts:
        - name: data
          mountPath: /var/lib/weaviate
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: weaviate-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: weaviate-service
spec:
  selector:
    app: weaviate
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: weaviate-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

Apply the configurations:

```bash
kubectl apply -f engram-deployment.yaml
kubectl apply -f weaviate-deployment.yaml
```

For external access, create an Ingress or use a LoadBalancer service type.

## Backup and Restore

### Creating Backups

To back up Engram v3 data, create a snapshot of the data directory:

```bash
# Stop the container first for consistency
docker stop engram

# Create a backup
docker run --rm -v engram-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/engram-backup-$(date +%Y%m%d).tar.gz /data

# Restart the container
docker start engram
```

For Weaviate data:

```bash
# Stop the container
docker stop weaviate

# Create a backup
docker run --rm -v weaviate-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/weaviate-backup-$(date +%Y%m%d).tar.gz /data

# Restart the container
docker start weaviate
```

### Restoring Backups

To restore from a backup:

```bash
# Stop the container
docker stop engram

# Restore the backup
docker run --rm -v engram-data:/data -v $(pwd):/backup alpine \
  sh -c "rm -rf /data/* && tar xzf /backup/engram-backup-20240101.tar.gz -C /"

# Restart the container
docker start engram
```

For Weaviate:

```bash
# Stop the container
docker stop weaviate

# Restore the backup
docker run --rm -v weaviate-data:/data -v $(pwd):/backup alpine \
  sh -c "rm -rf /data/* && tar xzf /backup/weaviate-backup-20240101.tar.gz -C /"

# Restart the container
docker start weaviate
```

## Multi-Environment Setup

### Development Environment

```bash
docker-compose -f docker-compose.dev.yml up -d
```

Example `docker-compose.dev.yml`:

```yaml
version: '3.8'

services:
  engram:
    image: engram:v3
    container_name: engram-dev
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - LOG_LEVEL=debug
      - VECTOR_SEARCH_ENABLED=false
      - TELEMETRY_ENABLED=false
    restart: unless-stopped
```

### Staging Environment

```bash
docker-compose -f docker-compose.staging.yml up -d
```

### Production Environment

For production, use Kubernetes or Docker Compose with proper monitoring and scaling.

## Performance Tuning

Optimize Engram v3 for your specific workload:

### Memory Configuration

For high-throughput deployments, allocate more memory:

```bash
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  --memory=4g \
  --memory-reservation=2g \
  -e STORAGE_OPTIMIZED=true \
  engram:v3
```

### Storage Configuration

For optimal performance, use SSD-backed volumes:

```bash
# Create a volume with specific driver
docker volume create --driver local \
  --opt type=ext4 \
  --opt device=/dev/ssd-device \
  engram-data
```

The optimized BadgerDB configuration provides significantly better performance for high-write workloads. Enable it with:

```bash
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  -e STORAGE_OPTIMIZED=true \
  engram:v3
```

### Network Configuration

For high-volume installations, optimize network settings:

```bash
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  --sysctl net.core.somaxconn=4096 \
  --sysctl net.ipv4.tcp_max_syn_backlog=4096 \
  engram:v3
```

### Vector Search Configuration

For optimal vector search performance:

```bash
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  -e VECTOR_SEARCH_ENABLED=true \
  -e WEAVIATE_URL=http://weaviate:8080 \
  -e EMBEDDING_BATCH_SIZE=10 \  # Batch size for embedding requests
  engram:v3
```

## Security Considerations

### Running Behind a Reverse Proxy

For production deployments, run Engram v3 behind a reverse proxy like Nginx:

```nginx
server {
    listen 443 ssl;
    server_name engram.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket support
    location /stream {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### Environment Variables

For sensitive configuration, use environment variables rather than command-line arguments:

```bash
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  -e EMBEDDING_KEY="your-openai-api-key" \
  -e WEAVIATE_API_KEY="your-weaviate-api-key" \
  engram:v3
```

For Kubernetes, use Secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: engram-secrets
type: Opaque
data:
  embedding-key: base64-encoded-api-key
  weaviate-api-key: base64-encoded-api-key
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: engram
spec:
  template:
    spec:
      containers:
      - name: engram
        env:
        - name: EMBEDDING_KEY
          valueFrom:
            secretKeyRef:
              name: engram-secrets
              key: embedding-key
        - name: WEAVIATE_API_KEY
          valueFrom:
            secretKeyRef:
              name: engram-secrets
              key: weaviate-api-key
```

## Troubleshooting

### Checking Container Logs

```bash
docker logs engram
```

### Inspecting Running Container

```bash
docker exec -it engram /bin/sh
```

### Common Issues

1. **Connection Refused**: Verify the container is running and ports are correctly mapped
2. **Data Persistence Issues**: Ensure volume is correctly configured
3. **High Memory Usage**: Adjust memory limits or optimize client connections
4. **Slow Performance**: Check storage backend and consider SSD-backed volumes
5. **Vector Search Not Working**: Verify Weaviate connection and API keys
6. **Embedding Service Errors**: Check embedding service URL and API key

### Debugging Vector Search

If vector search isn't working:

1. Check Weaviate connection:
   ```bash
   curl http://weaviate:8080/v1/.well-known/ready
   ```

2. Check embedding service configuration:
   ```bash
   docker exec -it engram env | grep EMBEDDING
   ```

3. Verify class creation in Weaviate:
   ```bash
   curl http://weaviate:8080/v1/schema
   ```

## Upgrading

To upgrade to a new version of Engram v3:

```bash
# Pull the latest image
docker pull engram:v3

# Stop the current container
docker stop engram

# Remove the current container
docker rm engram

# Start a new container with the latest image
docker run -d \
  --name engram \
  -p 8080:8080 \
  -v engram-data:/app/data \
  engram:v3
```

For Kubernetes:

```bash
kubectl set image deployment/engram engram=engram:v3-new
```