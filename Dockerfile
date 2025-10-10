# Build stage
FROM registry.redhat.io/ubi9/go-toolset:1.24 AS builder

# Set working directory
WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

# Final stage
FROM registry.redhat.io/ubi9/ubi-minimal:9.5

# Install ca-certificates for HTTPS requests
RUN microdnf update -y && \
    microdnf install -y ca-certificates && \
    microdnf clean all

# Create non-root user
RUN groupadd -g 1001 dora-metrics && \
    useradd -u 1001 -g dora-metrics -s /bin/bash -m dora-metrics

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /workspace/main .

# Change ownership to non-root user
RUN chown dora-metrics:dora-metrics main

# Switch to non-root user
USER dora-metrics

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]
