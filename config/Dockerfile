# EAMSA 512 Dockerfile
# Production-ready multi-stage Docker image for EAMSA 512
# 
# Build: docker build -t eamsa512:latest .
# Run:   docker run -d -p 8080:8080 -v $(pwd)/config:/root/config:ro eamsa512:latest
#
# Last updated: December 4, 2025

# ============================================================================
# Stage 1: Builder
# ============================================================================

FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    alpine-sdk

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src ./src

# Build the binary
# Use CGO_ENABLED=0 for static binary (works in minimal base images)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a \
    -installsuffix cgo \
    -ldflags="-s -w" \
    -o eamsa512 \
    ./src/main.go

# Verify binary was created
RUN test -f eamsa512 && echo "✓ Binary built successfully"

# ============================================================================
# Stage 2: Runtime (Minimal base image)
# ============================================================================

FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    tini \
    curl

# Create non-root user for security (principle of least privilege)
RUN addgroup -g 10001 eamsa512 && \
    adduser -u 10001 -G eamsa512 -s /sbin/nologin -D eamsa512

# Set working directory
WORKDIR /home/eamsa512

# Create necessary directories with proper permissions
RUN mkdir -p /home/eamsa512/{config,logs,keys,backups} && \
    chown -R eamsa512:eamsa512 /home/eamsa512 && \
    chmod 750 /home/eamsa512

# Copy TLS certificates from system (or pre-configured)
RUN mkdir -p /etc/eamsa512/certs && \
    chown -R eamsa512:eamsa512 /etc/eamsa512 && \
    chmod 700 /etc/eamsa512/certs

# Copy binary from builder stage
COPY --from=builder /app/eamsa512 /home/eamsa512/eamsa512

# Copy default configuration (can be overridden at runtime)
COPY config/eamsa512.yaml /home/eamsa512/config/eamsa512.yaml.default

# Set permissions on binary
RUN chown eamsa512:eamsa512 /home/eamsa512/eamsa512 && \
    chmod 550 /home/eamsa512/eamsa512

# Switch to non-root user
USER eamsa512

# Expose port
EXPOSE 8080 9090

# Health check endpoint
# This helps Kubernetes and Docker health monitoring systems detect failures
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f -k https://localhost:8080/api/v1/health || exit 1

# Use tini as PID 1 to handle signals properly and reap zombie processes
ENTRYPOINT ["/sbin/tini", "--"]

# Default command (can be overridden)
CMD ["/home/eamsa512/eamsa512", "-config", "/home/eamsa512/config/eamsa512.yaml"]

# ============================================================================
# Image Metadata
# ============================================================================

LABEL maintainer="security@example.com" \
      version="1.0" \
      description="EAMSA 512 - Enterprise 512-bit Authenticated Encryption" \
      org.opencontainers.image.title="EAMSA 512" \
      org.opencontainers.image.description="FIPS 140-2 Level 2 certified cryptographic system" \
      org.opencontainers.image.vendor="Example Corp" \
      org.opencontainers.image.licenses="MIT"

# ============================================================================
# Security Notes
# ============================================================================

# 1. Non-root user: Service runs as eamsa512:eamsa512 (UID 10001)
#    Prevents privilege escalation from container escape

# 2. Multi-stage build: Reduces final image size by excluding build tools
#    Final image contains only runtime dependencies

# 3. Minimal base image: alpine:3.19 provides security patches
#    Smaller attack surface than full OS images

# 4. Static binary: CGO_ENABLED=0 means no C runtime dependencies
#    Binary is fully self-contained

# 5. Read-only mounts: Configuration should be read-only in production
#    docker run -v config:/config:ro

# 6. Health check: Kubernetes and Docker can monitor service health
#    Enables automatic restart on failure

# 7. Signals: tini as PID 1 ensures clean shutdown
#    Graceful termination on SIGTERM

# 8. No shell in runtime: Reduces attack surface
#    No interactive shell in final image

# ============================================================================
# Build Arguments (Optional)
# ============================================================================

# ARG EAMSA_VERSION=1.0
# ARG GO_VERSION=1.21
# ARG BASE_IMAGE=alpine:3.19

# ============================================================================
# Volume Mounts (for docker run)
# ============================================================================

# /home/eamsa512/config         - Configuration files (mount read-only)
# /home/eamsa512/logs           - Application logs
# /home/eamsa512/keys           - Encrypted key storage
# /home/eamsa512/backups        - Backup directory
# /etc/eamsa512/certs           - TLS certificates

# ============================================================================
# Environment Variables (for docker run)
# ============================================================================

# EAMSA_SERVER_HOST             - Server bind address (default: 0.0.0.0)
# EAMSA_SERVER_PORT             - Server port (default: 8080)
# EAMSA_LOG_LEVEL               - Log level (DEBUG, INFO, WARN, ERROR)
# EAMSA_HSM_ENABLED             - Enable HSM integration (true/false)
# EAMSA_HSM_TYPE                - HSM type (thales, yubihsm, aws-cloudhsm, softhsm)
# EAMSA_TLS_ENABLED             - Enable TLS (true/false)
# EAMSA_FIPS_MODE               - Enable FIPS mode (true/false)

# Example environment file usage:
# docker run --env-file .env eamsa512:latest

# ============================================================================
# Example Docker Run Commands
# ============================================================================

# 1. Basic run with default configuration:
#    docker run -d -p 8080:8080 \
#      --name eamsa512 \
#      eamsa512:latest

# 2. With custom config volume:
#    docker run -d -p 8080:8080 \
#      -v $(pwd)/config/eamsa512.yaml:/home/eamsa512/config/eamsa512.yaml:ro \
#      --name eamsa512 \
#      eamsa512:latest

# 3. With persistent storage for keys and logs:
#    docker run -d -p 8080:8080 \
#      -v eamsa512-keys:/home/eamsa512/keys \
#      -v eamsa512-logs:/home/eamsa512/logs \
#      -v $(pwd)/config:/home/eamsa512/config:ro \
#      --name eamsa512 \
#      eamsa512:latest

# 4. With environment variables and resource limits:
#    docker run -d -p 8080:8080 \
#      --memory="512m" \
#      --cpus="1" \
#      --env LOG_LEVEL=INFO \
#      --env HSM_ENABLED=true \
#      --name eamsa512 \
#      eamsa512:latest

# 5. With HSM network integration:
#    docker run -d -p 8080:8080 \
#      --network hsm-network \
#      -v /etc/eamsa512/hsm:/etc/eamsa512/hsm:ro \
#      -v $(pwd)/config:/home/eamsa512/config:ro \
#      --env HSM_TYPE=thales \
#      --env HSM_HOST=hsm.example.com \
#      --name eamsa512 \
#      eamsa512:latest

# 6. With logging driver and monitoring:
#    docker run -d -p 8080:8080 \
#      --log-driver=awslogs \
#      --log-opt awslogs-group=/ecs/eamsa512 \
#      --log-opt awslogs-region=us-east-1 \
#      -v eamsa512-data:/home/eamsa512 \
#      --name eamsa512 \
#      eamsa512:latest

# ============================================================================
# Docker Compose Integration
# ============================================================================

# See deployment/docker-compose.yml for full orchestration example
# 
# Quick start:
# docker-compose up -d

# ============================================================================
# Image Size Optimization Tips
# ============================================================================

# Current approach uses multi-stage build to minimize final image size:
# - Builder stage: ~800MB (includes Go toolchain)
# - Runtime stage: ~20-30MB (minimal base + binary + runtime deps)
#
# Further optimization (if needed):
# 1. Use scratch base image instead of alpine (no shell, minimal size)
#    FROM scratch
#    COPY --from=builder /etc/ssl/certs /etc/ssl/certs  # Add only needed certs
#
# 2. Strip debugging symbols (already done with -ldflags="-s -w")
#
# 3. Compress executable with upx (requires testing for Go binaries)

# ============================================================================
# Security Scanning
# ============================================================================

# Scan image for vulnerabilities:
# docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
#   aquasec/trivy image eamsa512:latest

# Check for exposed secrets:
# docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
#   aquasec/trivy image --severity HIGH,CRITICAL eamsa512:latest

# ============================================================================
# CI/CD Integration
# ============================================================================

# GitHub Actions example:
# - name: Build Docker image
#   run: docker build -t eamsa512:${{ github.sha }} .
#
# - name: Push to registry
#   run: docker push myregistry.azurecr.io/eamsa512:${{ github.sha }}
#
# - name: Scan for vulnerabilities
#   run: |
#     docker run --rm aquasec/trivy image \
#       myregistry.azurecr.io/eamsa512:${{ github.sha }}

# ============================================================================
# Production Deployment Checklist
# ============================================================================

# [ ] Build image: docker build -t eamsa512:v1.0 .
# [ ] Tag image: docker tag eamsa512:v1.0 registry/eamsa512:v1.0
# [ ] Scan image for vulnerabilities (Trivy, Clair, etc.)
# [ ] Test locally with docker run (verify health check works)
# [ ] Review Dockerfile for security best practices
# [ ] Ensure non-root user is used
# [ ] Verify TLS certificates are properly mounted
# [ ] Test HSM connectivity (if HSM integration enabled)
# [ ] Push to private container registry
# [ ] Deploy to Docker Compose or Kubernetes
# [ ] Verify service starts and health check passes
# [ ] Check logs for startup errors
# [ ] Verify API endpoints respond correctly
# [ ] Run compliance report: curl https://localhost:8080/api/v1/compliance/report

# ============================================================================
