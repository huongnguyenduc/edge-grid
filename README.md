# Edge AI Inference Marketplace

A decentralized marketplace for AI inference services with on-chain escrow and cryptographically signed results.

## 🏗️ Architecture

This monorepo contains:

- **Smart Contracts** (`packages/contracts/`) - Solidity contracts for node registry and job escrow
- **Frontend** (`apps/frontend/`) - Next.js application with wallet integration
- **Node Agent** (`apps/node-agent/`) - Go server for running AI models
- **Shared** (`packages/shared/`) - Common types and utilities

## 🚀 Quick Start

### Prerequisites

- Node.js 18+
- Go 1.21+
- pnpm
- Foundry (for smart contracts)

### Installation

```bash
# Install dependencies
pnpm install

# Install Foundry (if not already installed)
brew install foundry

# Initialize contracts
cd packages/contracts
forge install
```

### Development

#### Option 1: Local Development
```bash
# Start all development servers
make dev

# Or start individually:
make frontend-dev    # Frontend on :3000
make node-agent-run  # Node agent on :8080
```

#### Option 2: Docker Development
```bash
# Start all services with Docker Compose
make docker-dev

# Or manage individually:
make docker-build    # Build all images
make docker-up       # Start services in background
make docker-logs     # View logs
make docker-down     # Stop services
```

### Building

```bash
# Build all packages
make build

# Or build individually:
make contracts-build
make frontend-build
make node-agent-build
```

### Testing

```bash
# Run all tests
make test

# Or test individually:
make contracts-test
make node-agent-test
```

## 📁 Project Structure

```
hackathon/
├── apps/
│   ├── frontend/          # Next.js frontend
│   └── node-agent/        # Go server
├── packages/
│   ├── contracts/         # Solidity contracts
│   └── shared/           # Shared types/utils
├── prompts/              # AI prompt snippets
├── Makefile              # Development commands
├── AGENTS.md             # AI assistant repo guidance (replaces .cursorrules)
└── .cursorignore         # AI ignore patterns
```

## 🔧 Available Commands

### Root Commands
- `make help` - Show all available commands
- `make install` - Install all dependencies
- `make dev` - Start all development servers
- `make build` - Build all packages
- `make test` - Run all tests
- `make clean` - Clean build artifacts

### Contract Commands
- `make contracts-build` - Build smart contracts
- `make contracts-test` - Test smart contracts
- `make contracts-deploy NETWORK=sepolia` - Deploy to network

### Frontend Commands
- `make frontend-dev` - Start development server
- `make frontend-build` - Build for production

### Node Agent Commands
- `make node-agent-run` - Run server
- `make node-agent-build` - Build binary
- `make node-agent-test` - Run tests

### Docker Commands
- `make docker-build` - Build all Docker images
- `make docker-up` - Start services in background
- `make docker-down` - Stop all services
- `make docker-logs` - View logs from all services
- `make docker-clean` - Remove containers and images
- `make docker-dev` - Start development with hot reloading

## 🛠️ Development Workflow

### Smart Contracts

```bash
cd packages/contracts

# Build contracts
forge build

# Run tests
forge test -vvv

# Deploy to testnet
forge script script/Deploy.s.sol --rpc-url $RPC_URL --broadcast
```

### Frontend

```bash
cd apps/frontend

# Start development server
pnpm dev

# Build for production
pnpm build
```

### Node Agent

```bash
cd apps/node-agent

# Run server
go run cmd/server/main.go

# Build binary
go build -o bin/node-agent cmd/server/main.go

# Run tests
go test ./...
```

## 🔐 Environment Setup

Create `.env` files as needed:

### Frontend (.env.local)
```bash
NEXT_PUBLIC_CHAIN_ID=11155111
NEXT_PUBLIC_CONTRACT_ADDRESS=0x...
```

### Node Agent (.env)
```bash
PORT=8080
PRIVATE_KEY=0x...
CHAIN_ID=11155111
CONTRACT_ADDRESS=0x...
```

## 📚 Documentation

- [Architecture Documentation](./ARCHITECTURE.md) - Detailed technical specifications
- [Development Plan](./PLAN.md) - Hackathon development timeline
- [Prompt Snippets](./prompts/) - AI coding templates
- [Agents Guidance](./AGENTS.md) - Rules and context for AI assistants

## 🎯 Hackathon Goals

1. **Day 1**: Core infrastructure and smart contracts
2. **Day 2**: Frontend integration and node agent
3. **Day 3**: Polish, testing, and demo preparation

## 🤝 Contributing

This is a hackathon project. Follow the guidance in `AGENTS.md` for coding standards and use the prompt snippets in `prompts/` for common tasks.

## 📄 License

MIT License - see LICENSE file for details.
