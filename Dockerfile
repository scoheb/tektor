# Use Go base image for building
FROM golang:1.22-alpine AS builder

# Install git for go modules
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies (this layer will be cached)
RUN go mod download

# Copy source code
COPY . .

# Build the tektor binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o tektor main.go

# Final stage
FROM alpine:latest

# Cache busting argument
ARG CACHE_BUST=1

# Install required packages
RUN apk add --no-cache \
    bash \
    git \
    curl \
    jq \
    ca-certificates

# Copy the tektor binary from builder
COPY --from=builder /app/tektor /usr/local/bin/tektor

# Copy the entrypoint script
COPY entrypoint.sh /entrypoint.sh

# Make scripts executable
RUN chmod +x /entrypoint.sh /usr/local/bin/tektor

# Set the entrypoint
ENTRYPOINT ["/entrypoint.sh"] 
