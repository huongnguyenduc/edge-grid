# Stage 1: Build Go Binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod before to leverage Docker Cache
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary (CGO_ENABLED=0 to run static link, no OS library dependency)
RUN CGO_ENABLED=0 GOOS=linux go build -o edge-node cmd/node/main.go

# Stage 2: Final Runtime Image (Use Alpine for lightweight)
FROM alpine:latest

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/edge-node .

# Copy file wasm sample for demo (Important)
COPY hello.wasm .

# Expose P2P port and WebSocket port
EXPOSE 4001 8001

# Run app
ENTRYPOINT ["./edge-node"]