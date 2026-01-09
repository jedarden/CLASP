# Build stage
FROM golang:1.25-alpine AS builder

# Build argument for version
ARG VERSION=dev

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version injected
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /clasp ./cmd/clasp

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -u 1000 clasp
USER clasp

WORKDIR /app

# Copy binary from builder
COPY --from=builder /clasp /app/clasp

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the proxy
ENTRYPOINT ["/app/clasp"]
CMD ["-port", "8080"]
