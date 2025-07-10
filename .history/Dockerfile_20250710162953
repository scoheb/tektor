# Use Go base image for building
FROM golang:1.22-alpine AS builder

# Install git and ca-certificates for go modules
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build tektor binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tektor main.go

# Use Alpine for the final image
FROM alpine:3.19

# Install required packages for the action
RUN apk add --no-cache \
    bash \
    git \
    curl \
    jq \
    yq \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Copy the tektor binary from builder
COPY --from=builder /app/tektor /usr/local/bin/tektor

# Copy the entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set the entrypoint
ENTRYPOINT ["/entrypoint.sh"] 
