# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------
# Stage 1: Build the static binary
# ---------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source tree
COPY . .

# Build a statically linked binary with no CGO dependencies
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o awm-cli ./cmd/awm-cli

# ---------------------------------------------------------------------
# Stage 2: Create a minimal runtime image
# ---------------------------------------------------------------------
FROM alpine:3.22

# Install runtime dependencies only
RUN apk add --no-cache ca-certificates tzdata && \
    update-ca-certificates

# Create a non‑root user for security
RUN adduser -D -u 1000 -h /app awm
USER awm
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder --chown=awm:awm /build/awm-cli /app/awm-cli

# Create a directory for configuration files
RUN mkdir -p /app/configs && chown awm:awm /app/configs

# Default environment variables
ENV AWM_CONFIG=/app/configs/agent.yaml

# Run the CLI
ENTRYPOINT ["/app/awm-cli"]