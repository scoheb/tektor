# Use Go base image for building
FROM golang:1.22-alpine AS builder

# Install git for go modules
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the tektor binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tektor main.go

# Final stage
FROM alpine:latest

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
