FROM golang:1.21-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o grafana-influx-exporter ./cmd/exporter

# Final stage
FROM alpine:3.18

# Install required packages
RUN apk add --no-cache ca-certificates curl bash

# Create app user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Create necessary directories
RUN mkdir -p /app/logs && \
    chown -R appuser:appgroup /app

WORKDIR /app

# Copy binary and entrypoint
COPY --from=builder /app/grafana-influx-exporter .
COPY docker-entrypoint.sh /app/

# Create default config
COPY config.yaml.example /app/config.yaml.example

# Make files executable and fix line endings
RUN chmod +x /app/grafana-influx-exporter /app/docker-entrypoint.sh && \
    sed -i 's/\r$//' /app/docker-entrypoint.sh

# Switch to app user
USER appuser

EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Start the application
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD []