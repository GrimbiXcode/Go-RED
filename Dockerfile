# Go—RED Dockerfile
# Multi-stage build for smaller final image

# Build stage
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Set environment variables
ENV GO111MODULE=on     CGO_ENABLED=0     GOOS=linux     GOARCH=amd64

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o go-red -v ./cmd/go-red

# Runtime stage
FROM alpine:3.18

# Set working directory
WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/go-red .

# Copy WebUI static files (will be built separately)
COPY --from=builder /app/web/dist ./web/dist

# Create directories for data and plugins
RUN mkdir -p data/flows plugins/go plugins/js

# Copy default flows and plugins (if any)
COPY --from=builder /app/plugins ./plugins

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Change ownership of the app directory
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Set environment variables
ENV PORT=8080     DATA_DIR=/app/data     PLUGIN_DIR=/app/plugins

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3     CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/flows || exit 1

# Entry point
ENTRYPOINT ["./go-red"]
CMD ["-port", "8080", "-data-dir", "/app/data", "-plugin-dir", "/app/plugins", "-web-dir", "/app/web/dist"]
