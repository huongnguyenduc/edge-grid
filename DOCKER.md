# Docker Setup Guide

This guide explains how to use Docker for the Edge AI Inference Marketplace project.

## 🐳 Quick Start

### Prerequisites
- Docker Desktop installed
- Docker Compose v2.0+

### Start Development Environment
```bash
# Start all services with hot reloading
make docker-dev

# Or start in background
make docker-up
```

### Access Services
- **Frontend**: http://localhost:3000
- **Node Agent**: http://localhost:8080
- **Contracts**: Available in contracts container

## 📦 First Run Notes
- Cold installs may take several minutes while images and dependencies are downloaded.
- On Apple Silicon (ARM), native modules (e.g., `bufferutil`, `secp256k1`) might build from source; we install Python, make, and g++ in the frontend image to support this.

## 📦 Services

### Frontend Service
- **Image**: Built from `apps/frontend/Dockerfile`
- **Port**: 3000
- **Features**: Hot reloading, TypeScript support
- **Dependencies**: Node.js 18, pnpm

### Node Agent Service
- **Image**: Built from `apps/node-agent/Dockerfile`
- **Port**: 8080
- **Features**: Hot reloading with Air, Go 1.21
- **Health Check**: GET /health

### Contracts Service
- **Image**: Built from `packages/contracts/Dockerfile`
- **Features**: Foundry, OpenZeppelin contracts
- **Commands**: forge build, forge test

## 🔧 Development Workflow

### Hot Reloading
Both frontend and node-agent support hot reloading:
- Frontend: Next.js dev server
- Node Agent: Air (Go hot reloader)

### Environment Variables
See `ENVIRONMENT.md` and create `.env` files as needed.

### Volume Mounts
- Source code is mounted for live editing
- `node_modules` and `go-mod-cache` are volumes for performance

## 🚀 Production Deployment

### Build Production Images
```bash
# Build all services for production
docker-compose -f docker-compose.yml -f docker-compose.prod.yml build
```

### Deploy to Cloud
```bash
# Example for AWS ECS or similar
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

## 🛠️ Troubleshooting

### Common Issues

**Port conflicts**: Change ports in `docker-compose.yml`
```yaml
ports:
  - "3001:3000"  # Use port 3001 instead
```

**Permission issues**: On Linux/macOS, ensure Docker has access to project directory

**Build failures**: Clean and rebuild
```bash
make docker-clean
make docker-build
```

### Debugging
```bash
# View logs
make docker-logs

# Access container shell
docker-compose exec frontend sh
docker-compose exec node-agent sh
docker-compose exec contracts sh

# Check service status
docker-compose ps
```

## 📋 Available Commands

| Command | Description |
|---------|-------------|
| `make docker-build` | Build all Docker images |
| `make docker-up` | Start services in background |
| `make docker-down` | Stop all services |
| `make docker-logs` | View logs from all services |
| `make docker-clean` | Remove containers and images |
| `make docker-dev` | Start development with hot reloading |

## 🔒 Security Notes

- Never commit real private keys to version control
- Use environment variables for sensitive data
- The `.dockerignore` file excludes sensitive files
- Production images should use non-root users

## 📚 Additional Resources

- Docker Compose Documentation
- Next.js Docker Guide
- Go Docker Best Practices

