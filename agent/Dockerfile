# Stage 1: Build Agent binary
FROM golang:1.23-alpine AS builder

WORKDIR /workspace

# Copy proto module
COPY proto/ proto/

# Copy agent source and dependencies
COPY agent/ agent/

WORKDIR /workspace/agent
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/provenops-agent ./cmd/agent/main.go

# Stage 2: Realistic Ubuntu 22.04 Linux Host Environment
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    iproute2 \
    procps \
    net-tools \
    systemd-sysv \
    nginx \
    && rm -rf /var/lib/apt/lists/*

# Setup ProvenOps directories and backward-compatible symlinks
RUN mkdir -p /var/lib/provenops/backups /var/lib/provenops/certs /var/log/provenops && \
    chmod 750 /var/lib/provenops && \
    ln -s /var/lib/provenops /var/lib/opspilot && \
    ln -s /var/log/provenops /var/log/opspilot

COPY --from=builder /bin/provenops-agent /usr/local/bin/provenops-agent
RUN ln -s /usr/local/bin/provenops-agent /usr/local/bin/opspilot-agent

EXPOSE 80 443

ENTRYPOINT ["/usr/local/bin/provenops-agent"]
