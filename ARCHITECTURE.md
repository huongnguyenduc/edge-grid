# Edge AI Inference Marketplace - Architecture Documentation

## System Overview

The Edge AI Inference Marketplace is a decentralized platform that enables users to purchase AI inference services from distributed nodes with on-chain escrow and cryptographically signed results. The system consists of four main components:

1. **Smart Contracts** (EVM-based blockchain)
2. **Frontend Application** (Next.js + Wagmi)
3. **Node Agent** (Go HTTP server)
4. **Supporting Infrastructure** (Storage, Indexing, Monitoring)

## Architecture Diagram

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │   Smart         │    │   Node Agent    │
│   (Next.js)     │◄──►│   Contracts     │◄──►│   (Go Server)   │
│                 │    │                 │    │                 │
│ • Wallet Connect│    │ • NodeRegistry  │    │ • ONNX Runtime  │
│ • Job Creation  │    │ • JobEscrow     │    │ • EIP-712 Sign  │
│ • Result Display│    │ • EIP-712 Verify│    │ • Rate Limiting │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Block         │    │   Indexing      │    │   Storage       │
│   Explorer      │    │   Service       │    │   Layer         │
│                 │    │                 │    │                 │
│ • Transaction   │    │ • Event Indexing│    │ • Image Upload  │
│   History       │    │ • Job Analytics │    │ • Result Cache  │
│ • Contract      │    │ • Node Metrics  │    │ • Model Storage │
│   Verification  │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Component Specifications

### 1. Smart Contracts

#### 1.1 NodeRegistry Contract

**Purpose**: Manages node registration and metadata

**Key Functions**:
```solidity
function register(string calldata uri, string[] calldata models, uint256 pricePerUnit) external
function update(string calldata uri, string[] calldata models, uint256 pricePerUnit) external
function info(address node) external view returns (string memory uri, string[] memory models, uint256 pricePerUnit)
function isRegistered(address node) external view returns (bool)
```

**Storage Structure**:
```solidity
struct NodeInfo {
    string uri;           // Node API endpoint
    string[] models;      // Supported model IDs
    uint256 pricePerUnit; // Price in wei per inference unit
    bool active;          // Registration status
    uint256 registeredAt; // Timestamp
}
```

**Events**:
- `NodeRegistered(address indexed node, string uri, string[] models, uint256 pricePerUnit)`
- `NodeUpdated(address indexed node, string uri, string[] models, uint256 pricePerUnit)`

#### 1.2 JobEscrow Contract

**Purpose**: Manages job lifecycle with escrow and settlement

**Key Functions**:
```solidity
function createJob(address node, address token, uint256 amount, uint256 deadline, string calldata modelId) external payable returns (uint256 jobId)
function settleJob(uint256 jobId, bytes32 resultHash, uint256 computeMs, uint256 score, uint256 nonce, uint256 sigDeadline, uint8 v, bytes32 r, bytes32 s) external
function cancelExpired(uint256 jobId) external
function jobs(uint256 jobId) external view returns (Job memory)
```

**Storage Structure**:
```solidity
enum Status { None, Open, Delivered, Settled, Canceled }

struct Job {
    address client;       // Job creator
    address node;         // Node address
    address token;        // Payment token (address(0) for native)
    uint256 amount;       // Escrowed amount
    uint256 deadline;     // Unix timestamp
    bytes32 resultHash;   // Result hash (set on settlement)
    Status status;        // Current status
    string modelId;       // Model identifier
    uint256 createdAt;    // Creation timestamp
}
```

**Security Features**:
- EIP-712 signature verification for results
- Nonce-based replay protection
- Deadline enforcement
- Native and ERC-20 token support

#### 1.3 EIP-712 Typed Data Structure

**Domain Separator**:
```typescript
{
  name: "EdgeAI",
  version: "1",
  chainId: number,
  verifyingContract: address
}
```

**Result Type**:
```typescript
{
  Result: [
    { name: "jobId", type: "uint256" },
    { name: "resultHash", type: "bytes32" },
    { name: "computeMs", type: "uint256" },
    { name: "score", type: "uint256" },
    { name: "nonce", type: "uint256" },
    { name: "deadline", type: "uint256" }
  ]
}
```

### 2. Frontend Application

#### 2.1 Technology Stack

- **Framework**: Next.js 14 with App Router
- **Blockchain Integration**: Wagmi v2 + viem
- **Wallet Connection**: RainbowKit + WalletConnect
- **UI Components**: Tailwind CSS + shadcn/ui
- **State Management**: Zustand
- **HTTP Client**: Axios
- **Form Handling**: React Hook Form + Zod validation

#### 2.2 Project Structure

```
frontend/
├── app/
│   ├── (auth)/
│   │   └── connect/
│   ├── dashboard/
│   │   ├── jobs/
│   │   └── nodes/
│   ├── create-job/
│   └── layout.tsx
├── components/
│   ├── ui/           # shadcn/ui components
│   ├── wallet/       # Wallet connection
│   ├── job/          # Job-related components
│   └── node/         # Node-related components
├── lib/
│   ├── contracts/    # Contract ABIs and addresses
│   ├── hooks/        # Custom React hooks
│   ├── utils/        # Utility functions
│   └── types/        # TypeScript types
├── stores/           # Zustand stores
└── public/
```

#### 2.3 Key Components

**WalletProvider**:
```typescript
interface WalletContextType {
  address: Address | undefined;
  isConnected: boolean;
  chain: Chain | undefined;
  connect: () => void;
  disconnect: () => void;
}
```

**JobStore**:
```typescript
interface JobStore {
  jobs: Job[];
  createJob: (params: CreateJobParams) => Promise<void>;
  settleJob: (jobId: number, signature: Signature) => Promise<void>;
  cancelJob: (jobId: number) => Promise<void>;
  fetchJobs: () => Promise<void>;
}
```

**NodeStore**:
```typescript
interface NodeStore {
  nodes: NodeInfo[];
  fetchNodes: () => Promise<void>;
  registerNode: (params: RegisterNodeParams) => Promise<void>;
}
```

#### 2.4 API Integration

**Contract Interactions**:
```typescript
// Job creation
const { writeContract } = useWriteContract();
await writeContract({
  address: JOB_ESCROW_ADDRESS,
  abi: JobEscrowABI,
  functionName: 'createJob',
  args: [nodeAddress, tokenAddress, amount, deadline, modelId],
  value: isNative ? amount : undefined
});

// Job settlement
await writeContract({
  address: JOB_ESCROW_ADDRESS,
  abi: JobEscrowABI,
  functionName: 'settleJob',
  args: [jobId, resultHash, computeMs, score, nonce, sigDeadline, v, r, s]
});
```

### 3. Node Agent (Go Server)

#### 3.1 Technology Stack

- **Runtime**: Go 1.21+
- **HTTP Framework**: Gin
- **ONNX Runtime**: CGO bindings
- **Ethereum Integration**: go-ethereum
- **Rate Limiting**: ulule/limiter
- **Configuration**: godotenv
- **Logging**: logrus
- **Testing**: testify

#### 3.2 Project Structure

```
node-agent/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── handlers/
│   │   ├── inference.go
│   │   ├── health.go
│   │   └── metrics.go
│   ├── models/
│   │   ├── onnx.go
│   │   └── registry.go
│   ├── crypto/
│   │   ├── eip712.go
│   │   └── signer.go
│   ├── middleware/
│   │   ├── rate_limit.go
│   │   ├── cors.go
│   │   └── auth.go
│   ├── config/
│   │   └── config.go
│   └── storage/
│       ├── cache.go
│       └── filesystem.go
├── pkg/
│   ├── types/
│   │   └── inference.go
│   └── utils/
│       └── validation.go
├── go.mod
├── go.sum
├── Dockerfile
└── docker-compose.yml
```

#### 3.3 Core Components

**InferenceHandler**:
```go
type InferenceHandler struct {
    modelRunner *models.ModelRunner
    signer      *crypto.EIP712Signer
    rateLimiter *limiter.Limiter
    cache       storage.Cache
    logger      *logrus.Logger
}

type InferenceRequest struct {
    JobID   uint64 `json:"jobId" binding:"required"`
    Image   string `json:"image" binding:"required"` // base64 encoded
    ModelID string `json:"modelId" binding:"required"`
}

type InferenceResponse struct {
    Payload     interface{} `json:"payload"`
    ResultHash  string      `json:"resultHash"`
    Signature   string      `json:"signature"`
    Telemetry   Telemetry   `json:"telemetry"`
}

type Telemetry struct {
    ComputeMs uint64  `json:"computeMs"`
    Score     float64 `json:"score"`
    MemoryMB  uint64  `json:"memoryMB"`
}
```

**ModelRunner**:
```go
type ModelRunner struct {
    models map[string]*onnxruntime.Session
    mutex  sync.RWMutex
}

func (mr *ModelRunner) LoadModel(modelID string, modelPath string) error
func (mr *ModelRunner) RunInference(modelID string, input []float32) (*InferenceResult, error)
func (mr *ModelRunner) GetSupportedModels() []string
```

**EIP712Signer**:
```go
type EIP712Signer struct {
    privateKey *ecdsa.PrivateKey
    chainID    *big.Int
    domain     eip712.Domain
}

func (s *EIP712Signer) SignResult(jobID uint64, resultHash string, computeMs uint64, score float64, nonce uint64, deadline uint64) (string, error)
func (s *EIP712Signer) VerifySignature(signature string, data interface{}) (bool, error)
```

#### 3.4 API Endpoints

**POST /api/v1/infer**
```json
{
  "jobId": 123,
  "image": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQ...",
  "modelId": "mobilenet_v2"
}
```

**Response**:
```json
{
  "payload": {
    "predictions": [
      {"class": "cat", "confidence": 0.95},
      {"class": "dog", "confidence": 0.03}
    ]
  },
  "resultHash": "0x1234...",
  "signature": "0xabcd...",
  "telemetry": {
    "computeMs": 150,
    "score": 0.95,
    "memoryMB": 256
  }
}
```

**GET /api/v1/health**
```json
{
  "status": "healthy",
  "models": ["mobilenet_v2", "resnet50"],
  "uptime": "2h30m",
  "version": "1.0.0"
}
```

**GET /api/v1/metrics**
```json
{
  "totalInferences": 1250,
  "averageLatency": 145,
  "successRate": 0.98,
  "activeModels": 2
}
```

#### 3.5 Configuration

**Environment Variables**:
```bash
# Server
PORT=8080
HOST=0.0.0.0

# Ethereum
PRIVATE_KEY=0x...
CHAIN_ID=11155111
CONTRACT_ADDRESS=0x...

# Models
MODEL_PATH=/app/models
SUPPORTED_MODELS=mobilenet_v2,resnet50

# Rate Limiting
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=1m

# Storage
CACHE_SIZE=1000
CACHE_TTL=1h
```

### 4. Supporting Infrastructure

#### 4.1 Storage Layer

**Image Storage**:
- Direct upload to node (streaming)
- Temporary local storage with cleanup
- Optional: IPFS integration for decentralized storage

**Model Storage**:
- Local filesystem with model registry
- Version management and updates
- Checksum verification

**Cache Layer**:
- In-memory LRU cache for frequent requests
- Redis for distributed caching (optional)
- Result caching with TTL

#### 4.2 Indexing Service

**Event Indexing**:
```typescript
interface IndexedEvent {
  blockNumber: number;
  transactionHash: string;
  eventName: string;
  args: any;
  timestamp: number;
}

interface JobIndex {
  jobId: number;
  client: string;
  node: string;
  status: string;
  createdAt: number;
  settledAt?: number;
  amount: string;
}
```

**Analytics**:
- Job completion rates
- Node performance metrics
- Revenue tracking
- Error analysis

#### 4.3 Monitoring & Observability

**Metrics Collection**:
- Request latency and throughput
- Error rates and types
- Resource utilization
- Blockchain interaction metrics

**Logging**:
- Structured logging with correlation IDs
- Request/response logging
- Error tracking and alerting
- Audit trail for security events

**Health Checks**:
- Node availability monitoring
- Contract interaction health
- Model loading status
- Database connectivity

## Security Architecture

### 1. Authentication & Authorization

**Node Authentication**:
- ECDSA key pair per node
- On-chain registration verification
- Nonce-based replay protection

**Client Authentication**:
- Wallet signature verification
- Transaction authorization
- Rate limiting per address

### 2. Data Protection

**Input Validation**:
- Image format and size validation
- Model ID verification
- Parameter sanitization

**Result Integrity**:
- Cryptographic hashing of results
- EIP-712 signature verification
- Nonce uniqueness enforcement

### 3. Network Security

**HTTPS/TLS**:
- End-to-end encryption
- Certificate management
- HSTS headers

**CORS Configuration**:
- Origin whitelist
- Method restrictions
- Header validation

**Rate Limiting**:
- Per-IP request limits
- Per-node capacity limits
- Burst protection

## Deployment Architecture

### 1. Smart Contracts

**Deployment Strategy**:
- Testnet deployment first
- Contract verification
- Proxy pattern for upgrades (optional)

**Network Configuration**:
- RPC endpoint configuration
- Gas price optimization
- Transaction retry logic

### 2. Frontend Deployment

**Build Process**:
- Static site generation
- Environment-specific builds
- Asset optimization

**Hosting Options**:
- Vercel (recommended for Next.js)
- Netlify
- AWS S3 + CloudFront

### 3. Node Agent Deployment

**Containerization**:
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o node-agent cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/node-agent .
COPY --from=builder /app/models ./models
CMD ["./node-agent"]
```

**Orchestration**:
- Docker Compose for local development
- Kubernetes for production
- Health checks and auto-restart

### 4. Infrastructure as Code

**Terraform Configuration**:
- VPC and networking
- Load balancers
- Database instances
- Monitoring setup

**CI/CD Pipeline**:
- Automated testing
- Contract deployment
- Frontend deployment
- Node agent deployment

## Performance Considerations

### 1. Scalability

**Horizontal Scaling**:
- Stateless node agents
- Load balancer distribution
- Database sharding (if needed)

**Vertical Scaling**:
- GPU acceleration for inference
- Memory optimization
- CPU optimization

### 2. Optimization Strategies

**Caching**:
- Result caching
- Model preloading
- CDN for static assets

**Batch Processing**:
- Multiple inference requests
- Bulk settlement transactions
- Batch event processing

**Async Processing**:
- Background job processing
- Event-driven architecture
- Message queues (optional)

## Development Workflow

### 1. Local Development

**Prerequisites**:
- Node.js 18+
- Go 1.21+
- Foundry or Hardhat
- Docker (optional)

**Setup Commands**:
```bash
# Smart contracts
cd contracts
forge install
forge build
forge test

# Frontend
cd frontend
npm install
npm run dev

# Node agent
cd node-agent
go mod download
go run cmd/server/main.go
```

### 2. Testing Strategy

**Unit Tests**:
- Contract unit tests
- Frontend component tests
- Go handler tests

**Integration Tests**:
- End-to-end workflow tests
- Contract interaction tests
- API integration tests

**Load Tests**:
- Concurrent request testing
- Stress testing
- Performance benchmarking

### 3. Code Quality

**Linting & Formatting**:
- Solidity: solhint, prettier-plugin-solidity
- TypeScript: ESLint, Prettier
- Go: golangci-lint, gofmt

**Security Scanning**:
- Contract security audits
- Dependency vulnerability scanning
- Static code analysis

## Future Enhancements

### 1. Advanced Features

**Multi-Model Support**:
- Dynamic model loading
- Model versioning
- A/B testing

**Advanced Pricing**:
- Per-token pricing
- Dynamic pricing
- Auction-based pricing

**Reputation System**:
- SBT-based reputation
- Performance scoring
- Slashing mechanisms

### 2. Technical Improvements

**Privacy**:
- zkML integration
- Homomorphic encryption
- Secure multi-party computation

**Interoperability**:
- Cross-chain support
- Layer 2 integration
- Bridge mechanisms

**Analytics**:
- Advanced metrics
- Machine learning insights
- Predictive analytics

This architecture document provides a comprehensive technical specification for implementing the Edge AI Inference Marketplace. Each component is designed to be modular, scalable, and secure while maintaining simplicity for rapid development during the hackathon timeframe.

