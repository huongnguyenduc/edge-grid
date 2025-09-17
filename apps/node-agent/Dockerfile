# Node Agent Dockerfile
FROM golang:1.21-alpine AS base

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY apps/node-agent/go.mod apps/node-agent/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY apps/node-agent ./

# Development stage
FROM base AS development
RUN go install github.com/cosmtrek/air@v1.40.4
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

# Production stage
FROM base AS production
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o node-agent cmd/server/main.go
EXPOSE 8080
CMD ["./node-agent"]
