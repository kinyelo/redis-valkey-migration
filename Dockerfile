# Multi-stage build for Redis to Valkey Migration Tool

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG VERSION=1.0.0
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN make build VERSION=${VERSION} GIT_COMMIT=${GIT_COMMIT} BUILD_DATE=${BUILD_DATE}

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S migration && \
    adduser -u 1001 -S migration -G migration

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/build/redis-valkey-migration /usr/local/bin/redis-valkey-migration

# Create directories for logs and config
RUN mkdir -p /app/logs /app/config && \
    chown -R migration:migration /app

# Switch to non-root user
USER migration

# Set default environment variables
ENV LOG_LEVEL=info
ENV REDIS_HOST=localhost
ENV REDIS_PORT=6379
ENV VALKEY_HOST=localhost
ENV VALKEY_PORT=6380

# Expose no ports (this is a client tool)

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD redis-valkey-migration version || exit 1

# Default command
ENTRYPOINT ["redis-valkey-migration"]
CMD ["--help"]

# Labels
LABEL maintainer="Redis to Valkey Migration Tool"
LABEL version="${VERSION}"
LABEL description="A tool to migrate data from Redis to Valkey databases"