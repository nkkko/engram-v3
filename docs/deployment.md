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

### Docker Compose Setup

For a more complete deployment, use Docker Compose:

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
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

  # Optional monitoring with Prometheus
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    restart: unless-stopped
    depends_on:
      - engram

volumes:
  engram-data:
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

Apply the configuration:

```bash
kubectl apply -f engram-deployment.yaml
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

## Multi-Environment Setup

### Development Environment

```bash
docker run -d \
  --name engram-dev \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e LOG_LEVEL=debug \
  engram:v3
```

### Staging Environment

```bash
docker run -d \
  --name engram-staging \
  -p 8080:8080 \
  -v engram-staging-data:/app/data \
  -e LOG_LEVEL=info \
  engram:v3
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
  -e JWT_SECRET="your-secret-key" \
  engram:v3
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