# Stage 1: Build
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev openssl-dev

# Copy source
COPY . .

# Download modules
RUN go mod download

# Build application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o eamsa512 .

# Stage 2: Runtime
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs openssl tzdata

# Create app user
RUN addgroup -S eamsa512 && adduser -S eamsa512 -G eamsa512

# Setup directories
RUN mkdir -p /app /etc/eamsa512 /var/log/eamsa512 /var/lib/eamsa512
RUN chown -R eamsa512:eamsa512 /app /etc/eamsa512 /var/log/eamsa512 /var/lib/eamsa512

WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=eamsa512:eamsa512 /build/eamsa512 .

# Copy config template
COPY --chown=eamsa512:eamsa512 config/ /etc/eamsa512/

# Switch to non-root user
USER eamsa512

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider https://localhost:8080/api/v1/health || exit 1

# Expose ports
EXPOSE 8080 9090

# Set environment
ENV CONFIG_DIR=/etc/eamsa512
ENV LOG_DIR=/var/log/eamsa512
ENV DATA_DIR=/var/lib/eamsa512

# Run application
ENTRYPOINT ["./eamsa512"]
