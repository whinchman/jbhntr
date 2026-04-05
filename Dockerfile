# Stage 1: Build
FROM golang:1.25-bookworm AS builder

WORKDIR /src

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/jobhuntr ./cmd/jobhuntr

# Stage 2: Runtime
FROM debian:bookworm-slim

# Install CA certificates for HTTPS calls and chromium for go-rod PDF generation
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    chromium \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user
RUN useradd --system --no-create-home --uid 1001 appuser

WORKDIR /app

# Copy the compiled binary and config from the builder
COPY --from=builder /app/jobhuntr /app/jobhuntr
COPY config.yaml /app/config.yaml

# Pre-create writable output directory owned by appuser
RUN mkdir -p /app/output && chown appuser:appuser /app/output

USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/jobhuntr"]
CMD ["-config", "config.yaml"]
