.PHONY: help install dev build test clean contracts frontend node-agent docker

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install all dependencies
	pnpm install
	cd packages/contracts && forge install

dev: ## Start all development servers
	pnpm --filter @apps/frontend dev &
	cd apps/node-agent && go run cmd/server/main.go &
	@echo "Development servers started. Press Ctrl+C to stop all."

build: ## Build all packages
	pnpm build
	cd packages/contracts && forge build
	cd apps/node-agent && go build -o bin/node-agent cmd/server/main.go

test: ## Run all tests
	pnpm test
	cd packages/contracts && forge test
	cd apps/node-agent && go test ./...

clean: ## Clean all build artifacts
	pnpm clean
	cd packages/contracts && forge clean
	cd apps/node-agent && rm -rf bin/

contracts: ## Work with contracts
	@echo "Available commands:"
	@echo "  make contracts-build  - Build contracts"
	@echo "  make contracts-test   - Test contracts"
	@echo "  make contracts-deploy - Deploy contracts (set NETWORK env var)"

contracts-build: ## Build smart contracts
	cd packages/contracts && forge build

contracts-test: ## Test smart contracts
	cd packages/contracts && forge test -vvv

contracts-deploy: ## Deploy contracts to network
	@if [ -z "$(NETWORK)" ]; then echo "Please set NETWORK env var (e.g., NETWORK=sepolia)"; exit 1; fi
	cd packages/contracts && forge script script/Deploy.s.sol --rpc-url $(NETWORK) --broadcast

frontend: ## Work with frontend
	@echo "Available commands:"
	@echo "  make frontend-dev   - Start frontend dev server"
	@echo "  make frontend-build - Build frontend for production"

frontend-dev: ## Start frontend development server
	pnpm --filter @apps/frontend dev

frontend-build: ## Build frontend for production
	pnpm --filter @apps/frontend build

node-agent: ## Work with node agent
	@echo "Available commands:"
	@echo "  make node-agent-run   - Run node agent server"
	@echo "  make node-agent-build - Build node agent binary"
	@echo "  make node-agent-test  - Run node agent tests"

node-agent-run: ## Run node agent server
	cd apps/node-agent && go run cmd/server/main.go

node-agent-build: ## Build node agent binary
	cd apps/node-agent && go build -o bin/node-agent cmd/server/main.go

node-agent-test: ## Run node agent tests
	cd apps/node-agent && go test ./...

docker: ## Work with Docker
	@echo "Available commands:"
	@echo "  make docker-build   - Build all Docker images"
	@echo "  make docker-up      - Start all services with Docker Compose"
	@echo "  make docker-down    - Stop all Docker services"
	@echo "  make docker-logs    - View logs from all services"
	@echo "  make docker-clean   - Remove all containers and images"

docker-build: ## Build all Docker images
	docker-compose build

docker-up: ## Start all services with Docker Compose
	docker-compose up -d

docker-down: ## Stop all Docker services
	docker-compose down

docker-logs: ## View logs from all services
	docker-compose logs -f

docker-clean: ## Remove all containers and images
	docker-compose down --rmi all --volumes --remove-orphans

docker-dev: ## Start development environment with hot reloading
	docker-compose up --build
