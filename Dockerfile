# Build stage
FROM golang:1.23-bookworm AS builder

# Set environment variables
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies (Badger doesn't need any external libraries)
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -r engram && useradd -r -g engram engram

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire source code
COPY . .

# Build the application with CGO disabled for a more portable build
ENV CGO_ENABLED=0
RUN go build -o /app/engram /app/cmd/engram/main.go

# Create data directory
RUN mkdir -p /app/data && chown -R engram:engram /app/data

# Test stage - full implementation with required dependencies
FROM golang:1.23-bookworm AS test

# Set environment variables
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies (Badger doesn't need any external libraries)
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Copy app files from builder
COPY --from=builder /app /app
WORKDIR /app

# Set environment variables for tests
ENV CGO_ENABLED=0

# Create a test script to run all tests with emoji indicators
RUN echo '#!/bin/bash' > /app/run_tests.sh && \
    echo 'echo "🧪 Running all tests..."' >> /app/run_tests.sh && \
    echo 'core_packages="./internal/lockmanager ./internal/metrics ./internal/notifier ./internal/router ./pkg/client ./pkg/proto"' >> /app/run_tests.sh && \
    echo 'search_package="./internal/search"' >> /app/run_tests.sh && \
    echo 'storage_package="./internal/storage"' >> /app/run_tests.sh && \
    echo 'api_package="./internal/api"' >> /app/run_tests.sh && \
    echo 'echo "📦 Running core tests..."' >> /app/run_tests.sh && \
    echo 'if go test -v $core_packages; then' >> /app/run_tests.sh && \
    echo '  echo -e "\n✅ Core tests PASSED\n"' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo -e "\n❌ Core tests FAILED\n"' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo 'echo "🔍 Running Badger search tests..."' >> /app/run_tests.sh && \
    echo 'if go test -v $search_package; then' >> /app/run_tests.sh && \
    echo '  echo -e "\n✅ Badger search tests PASSED\n"' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo -e "\n❌ Badger search tests FAILED\n"' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo 'echo "💾 Running Badger storage tests..."' >> /app/run_tests.sh && \
    echo 'if go test -v $storage_package; then' >> /app/run_tests.sh && \
    echo '  echo -e "\n✅ Badger storage tests PASSED\n"' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo -e "\n❌ Badger storage tests FAILED\n"' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo 'echo "🌐 Running API tests..."' >> /app/run_tests.sh && \
    echo 'if go test -v $api_package; then' >> /app/run_tests.sh && \
    echo '  echo -e "\n✅ API tests PASSED\n"' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo -e "\n❌ API tests FAILED\n"' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo 'if [ -d "./tests" ]; then' >> /app/run_tests.sh && \
    echo '  echo "🧩 Running integration tests..."' >> /app/run_tests.sh && \
    echo '  if go test -v ./tests/...; then' >> /app/run_tests.sh && \
    echo '    echo -e "\n✅ Integration tests PASSED\n"' >> /app/run_tests.sh && \
    echo '  else' >> /app/run_tests.sh && \
    echo '    echo -e "\n❌ Integration tests FAILED\n"' >> /app/run_tests.sh && \
    echo '  fi' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo '' >> /app/run_tests.sh && \
    echo 'echo "🔄 Test Summary"' >> /app/run_tests.sh && \
    echo 'echo "=============="' >> /app/run_tests.sh && \
    echo 'passed=0' >> /app/run_tests.sh && \
    echo 'failed=0' >> /app/run_tests.sh && \
    echo '' >> /app/run_tests.sh && \
    echo 'if go test -v $core_packages &>/dev/null; then' >> /app/run_tests.sh && \
    echo '  echo "✅ Core tests: PASSED"' >> /app/run_tests.sh && \
    echo '  ((passed++))' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo "❌ Core tests: FAILED"' >> /app/run_tests.sh && \
    echo '  ((failed++))' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo '' >> /app/run_tests.sh && \
    echo 'if go test -v $search_package &>/dev/null; then' >> /app/run_tests.sh && \
    echo '  echo "✅ Search tests: PASSED"' >> /app/run_tests.sh && \
    echo '  ((passed++))' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo "❌ Search tests: FAILED"' >> /app/run_tests.sh && \
    echo '  ((failed++))' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo '' >> /app/run_tests.sh && \
    echo 'if go test -v $storage_package &>/dev/null; then' >> /app/run_tests.sh && \
    echo '  echo "✅ Storage tests: PASSED"' >> /app/run_tests.sh && \
    echo '  ((passed++))' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo "❌ Storage tests: FAILED"' >> /app/run_tests.sh && \
    echo '  ((failed++))' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo '' >> /app/run_tests.sh && \
    echo 'if go test -v $api_package &>/dev/null; then' >> /app/run_tests.sh && \
    echo '  echo "✅ API tests: PASSED"' >> /app/run_tests.sh && \
    echo '  ((passed++))' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo "❌ API tests: FAILED"' >> /app/run_tests.sh && \
    echo '  ((failed++))' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo '' >> /app/run_tests.sh && \
    echo 'if [ -d "./tests" ]; then' >> /app/run_tests.sh && \
    echo '  if go test -v ./tests/... &>/dev/null; then' >> /app/run_tests.sh && \
    echo '    echo "✅ Integration tests: PASSED"' >> /app/run_tests.sh && \
    echo '    ((passed++))' >> /app/run_tests.sh && \
    echo '  else' >> /app/run_tests.sh && \
    echo '    echo "❌ Integration tests: FAILED"' >> /app/run_tests.sh && \
    echo '    ((failed++))' >> /app/run_tests.sh && \
    echo '  fi' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    echo '' >> /app/run_tests.sh && \
    echo 'echo ""' >> /app/run_tests.sh && \
    echo 'echo "✅ PASSED: $passed test suites"' >> /app/run_tests.sh && \
    echo 'echo "❌ FAILED: $failed test suites"' >> /app/run_tests.sh && \
    echo 'if [ $failed -eq 0 ]; then' >> /app/run_tests.sh && \
    echo '  echo -e "\n🎉 ALL TESTS PASSED! 🎉\n"' >> /app/run_tests.sh && \
    echo '  exit 0' >> /app/run_tests.sh && \
    echo 'else' >> /app/run_tests.sh && \
    echo '  echo -e "\n❌ SOME TESTS FAILED ❌\n"' >> /app/run_tests.sh && \
    echo '  exit 1' >> /app/run_tests.sh && \
    echo 'fi' >> /app/run_tests.sh && \
    chmod +x /app/run_tests.sh

# Run the test script
CMD ["/app/run_tests.sh"]

# Runtime stage - small image for production
FROM debian:bookworm-slim AS runtime

# Set environment variables
ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies (Badger doesn't need any external libraries)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -r engram && useradd -r -g engram engram

# Set working directory and copy built binary
WORKDIR /app
COPY --from=builder /app/engram /app/
COPY --from=builder /app/data /app/data

# Make sure permissions are correct
RUN chown -R engram:engram /app

# Switch to non-root user
USER engram

# Expose HTTP port
EXPOSE 8080

# Command to run
CMD ["/app/engram"]