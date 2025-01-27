# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for private dependencies if needed
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/bot

# Final stage
FROM alpine:latest

WORKDIR /app

# Add timezone data
RUN apk add --no-cache tzdata

# Create history directory
RUN mkdir -p /app/history

# Copy the binary from builder
COPY --from=builder /app/main .
COPY --from=builder /app/config.yaml .

# Run the binary
CMD ["./main"] 