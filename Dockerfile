# CCOE Customer Contact Manager
FROM public.ecr.aws/docker/library/golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ccoe-customer-contact-manager .

# Final stage
FROM public.ecr.aws/docker/library/alpine:3.18

# Install ca-certificates for HTTPS requests and curl for health checks
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/ccoe-customer-contact-manager .

# Copy configuration file from builder stage
COPY --from=builder /app/config.json ./config.json

# Create directories for logs and data
RUN mkdir -p /app/logs /app/data && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose ports
EXPOSE 8080 8081 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ./ccoe-customer-contact-manager -mode=version || exit 1

# Set environment variables
ENV GIN_MODE=release
ENV LOG_LEVEL=info
ENV METRICS_PORT=9090
ENV HEALTH_PORT=8081
ENV API_PORT=8080

# Run the application
ENTRYPOINT ["./ccoe-customer-contact-manager"]