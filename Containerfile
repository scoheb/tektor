# Build stage
FROM registry.fedoraproject.org/fedora:40 AS builder

# Install Go and git
RUN dnf update -y && \
    dnf install -y golang git ca-certificates && \
    dnf clean all

# Set working directory
WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies with proxy and timeout optimizations
RUN go env -w GOPROXY=https://proxy.golang.org,direct && \
    go env -w GOSUMDB=sum.golang.org && \
    go mod download -x

# Copy source code
COPY . .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o tektor main.go

# Runtime stage
FROM registry.fedoraproject.org/fedora-minimal:40

# Install ca-certificates for HTTPS requests and git for git resolver functionality
RUN microdnf update -y && \
    microdnf install -y ca-certificates git && \
    microdnf clean all

# Create a non-root user
RUN groupadd -r tektor && useradd -r -g tektor tektor

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/tektor /usr/local/bin/tektor

# Change ownership to non-root user
RUN chown tektor:tektor /usr/local/bin/tektor

# Switch to non-root user
USER tektor

# Set the binary as executable
RUN chmod +x /usr/local/bin/tektor

# Expose any ports if needed (tektor is a CLI tool, so this might not be necessary)
# EXPOSE 8080

# Copy the action script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set the entrypoint
ENTRYPOINT ["/entrypoint.sh"] 
