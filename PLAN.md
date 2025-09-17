# Edge AI Inference Marketplace - Hackathon Plan

## Project Overview

**Objective**: Build a decentralized marketplace where users can purchase low-latency AI inference from nearby community nodes with on-chain escrow and cryptographically signed results.

**MVP Scope**: 
- One AI model (image classification)
- Simple node registry
- Escrowed job system
- Off-chain execution with signed results
- On-chain settlement/refund mechanism

## Architecture Overview

### Components
1. **Frontend**: Next.js + Wagmi/viem for wallet integration
2. **Smart Contracts**: Node registry and job escrow system
3. **Node Agent**: Go HTTP server running ONNX models
4. **Storage**: Direct uploads to nodes or temporary object storage

### Data Flow
1. Node registers on blockchain with metadata
2. Client selects node and creates escrowed job
3. Client sends image to node off-chain
4. Node returns result + EIP-712 signature
5. Client verifies signature and settles on-chain
6. Funds released to node or refunded on timeout

## Smart Contract Design

### NodeRegistry Contract
```solidity
interface INodeRegistry {
    event NodeRegistered(address indexed node, string uri, string[] models, uint256 pricePerUnit);
    event NodeUpdated(address indexed node, string uri, string[] models, uint256 pricePerUnit);

    function register(string calldata uri, string[] calldata models, uint256 pricePerUnit) external;
    function update(string calldata uri, string[] calldata models, uint256 pricePerUnit) external;
    function info(address node) external view returns (string memory uri, string[] memory models, uint256 pricePerUnit);
    function isRegistered(address node) external view returns (bool);
}
```

### JobEscrow Contract
```solidity
interface IJobEscrow {
    enum Status { None, Open, Delivered, Settled, Canceled }

    struct Job {
        address client;
        address node;
        address token;         // address(0) for native
        uint256 amount;        // fixed price for MVP
        uint256 deadline;      // unix seconds
        bytes32 resultHash;    // set on settle
        Status status;
    }

    event JobCreated(uint256 indexed jobId, address indexed client, address indexed node, address token, uint256 amount, uint256 deadline, string modelId);
    event JobSettled(uint256 indexed jobId, address indexed node, bytes32 resultHash);
    event JobCanceled(uint256 indexed jobId);

    function createJob(address node, address token, uint256 amount, uint256 deadline, string calldata modelId) external payable returns (uint256 jobId);
    function settleJob(uint256 jobId, bytes32 resultHash, uint256 computeMs, uint256 score, uint256 nonce, uint256 sigDeadline, uint8 v, bytes32 r, bytes32 s) external;
    function cancelExpired(uint256 jobId) external;
    function jobs(uint256 jobId) external view returns (Job memory);
}
```

### EIP-712 Typed Data Structure
```typescript
export const domain = (chainId: number, verifyingContract: `0x${string}`) => ({
  name: "EdgeAI",
  version: "1",
  chainId,
  verifyingContract
});

export const types = {
  Result: [
    { name: "jobId", type: "uint256" },
    { name: "resultHash", type: "bytes32" },   // keccak256(resultPayload)
    { name: "computeMs", type: "uint256" },    // telemetry
    { name: "score", type: "uint256" },        // model confidence
    { name: "nonce", type: "uint256" },        // replay protection
    { name: "deadline", type: "uint256" }
  ]
};
```

## Node Agent Implementation

### Technology Stack
- **Runtime**: Go HTTP server + ONNX Runtime (via CGO)
- **Model**: MobileNet/ResNet for image classification
- **Security**: ECDSA signing with node wallet
- **Dependencies**: `github.com/gin-gonic/gin`, `github.com/ethereum/go-ethereum`, `github.com/yalue/onnxruntime_go`

### API Endpoints
```
POST /infer
{
  "jobId": "uint256",
  "image": "base64_encoded_image",
  "modelId": "string"
}

Response:
{
  "payload": "inference_result_json",
  "resultHash": "bytes32",
  "signature": "EIP712_signature",
  "telemetry": {
    "computeMs": "uint256",
    "score": "uint256"
  }
}
```

### Security Features
- Rate limiting per IP (using `github.com/ulule/limiter/v3`)
- CORS configuration
- Wallet stored in environment variables
- Nonce-based replay protection
- Input validation and sanitization

### Go Implementation Details

#### Project Structure
```
node-agent/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── handlers/
│   │   └── inference.go
│   ├── models/
│   │   └── onnx.go
│   ├── crypto/
│   │   └── eip712.go
│   └── config/
│       └── config.go
├── go.mod
├── go.sum
└── Dockerfile
```

#### Key Dependencies
```go
// go.mod
module node-agent

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/ethereum/go-ethereum v1.13.5
    github.com/yalue/onnxruntime_go v1.8.0
    github.com/ulule/limiter/v3 v3.11.2
    github.com/joho/godotenv v1.5.1
)
```

#### Core Components
- **HTTP Server**: Gin framework for REST API
- **ONNX Runtime**: CGO bindings for model inference
- **EIP-712 Signing**: Ethereum signature generation
- **Rate Limiting**: Per-IP request limiting
- **Configuration**: Environment-based config management

#### Example Handler Structure
```go
type InferenceHandler struct {
    modelRunner *onnx.ModelRunner
    signer      *crypto.EIP712Signer
    rateLimiter *limiter.Limiter
}

func (h *InferenceHandler) Infer(c *gin.Context) {
    // Parse request
    // Run inference
    // Sign result with EIP-712
    // Return response
}
```

## Frontend Implementation

### Technology Stack
- **Framework**: Next.js 14
- **Blockchain**: Wagmi + viem
- **Wallet**: WalletConnect/RainbowKit
- **UI**: Tailwind CSS + shadcn/ui

### Key Pages
1. **Dashboard**: Node list, job history, wallet balance
2. **Create Job**: Upload image, select node, confirm escrow
3. **Job Status**: View results, settle/refund actions
4. **Node Registry**: Register/update node information

### User Flow
1. Connect wallet
2. Browse available nodes with pricing
3. Upload image and select node
4. Confirm escrow transaction
5. Wait for inference result
6. Review result and settle payment
7. View transaction on block explorer

## Development Timeline

### Day 1: Core Infrastructure (8-10 hours)
**Morning (4 hours)**
- [ ] Set up Foundry/Hardhat project
- [ ] Implement NodeRegistry contract
- [ ] Implement JobEscrow contract with EIP-712 verification
- [ ] Write unit tests for contracts
- [ ] Deploy to testnet

**Afternoon (4-6 hours)**
- [ ] Set up Next.js frontend with Wagmi
- [ ] Implement wallet connection
- [ ] Create node registry interface
- [ ] Build job creation flow
- [ ] Set up Go HTTP server node agent
- [ ] Implement basic ONNX model inference

### Day 2: Integration & Polish (8-10 hours)
**Morning (4 hours)**
- [ ] Complete E2E integration testing
- [ ] Implement job settlement flow
- [ ] Add timeout and refund functionality
- [ ] Create result display components
- [ ] Add error handling and loading states

**Afternoon (4-6 hours)**
- [ ] Implement job history and analytics
- [ ] Add telemetry display (compute time, confidence)
- [ ] Polish UI/UX with animations
- [ ] Add ERC-20 token support
- [ ] Implement nonce management
- [ ] Create demo script and test scenarios

### Day 3: Stretch Goals & Demo Prep (6-8 hours)
**Morning (3-4 hours)**
- [ ] Add second AI model (OCR or segmentation)
- [ ] Implement node reputation system
- [ ] Add per-unit pricing (megapixels, tokens)
- [ ] Create comprehensive documentation

**Afternoon (3-4 hours)**
- [ ] Record demo video
- [ ] Prepare presentation slides
- [ ] Test on venue WiFi
- [ ] Create backup demo scenarios
- [ ] Final bug fixes and optimizations

## Technical Implementation Details

### Contract Security Features
- EIP-712 signature verification for results
- Nonce-based replay protection
- Deadline enforcement for jobs
- Proper access controls
- Native and ERC-20 token support

### Node Agent Security
- ECDSA signature generation
- Rate limiting and CORS
- Environment variable configuration
- Input validation and sanitization

### Frontend Security
- Wallet signature verification
- Transaction confirmation dialogs
- Error boundary implementation
- Secure API communication

## Demo Script (3-minute presentation)

### Setup (30 seconds)
1. "This is Edge AI Marketplace - decentralized AI inference"
2. Show connected wallet with testnet funds
3. Display available nodes with pricing

### Core Demo (2 minutes)
1. Upload sample image (cat, dog, or custom)
2. Select node and confirm escrow transaction
3. Show inference progress with spinner
4. Display result with confidence score and preview
5. Click "Settle" to release payment to node
6. Show transaction confirmation and block explorer link

### Advanced Features (30 seconds)
1. Show job history and node earnings
2. Demonstrate refund flow with expired job
3. Highlight telemetry data (compute time, model info)

## Stretch Goals (if time permits)

### Advanced Features
- **Multi-model support**: OCR, object detection, segmentation
- **Dynamic pricing**: Per-megapixel or per-token pricing
- **Node reputation**: SBT-based reputation system
- **Gasless transactions**: Account abstraction for users
- **Cross-chain support**: Bridge rewards to L2

### Technical Enhancements
- **zkML integration**: Privacy-preserving inference
- **Batch processing**: Multiple images in one job
- **Real-time streaming**: WebSocket-based progress updates
- **Mobile app**: React Native companion app

## Risk Mitigation

### Technical Risks
- **Node availability**: Implement timeout and refund mechanisms
- **Model accuracy**: Use well-tested ONNX models
- **Network congestion**: Optimize gas usage and batch operations
- **Wallet connectivity**: Provide multiple wallet options

### Demo Risks
- **Network issues**: Prepare offline demo video
- **Transaction delays**: Use fast testnet with low gas
- **Browser compatibility**: Test on multiple browsers
- **Wallet connection**: Have backup wallet ready

## Success Metrics

### Technical Metrics
- [ ] Contracts deployed and verified on testnet
- [ ] E2E flow working with real transactions
- [ ] Node agent processing requests successfully
- [ ] Frontend responsive and user-friendly

### Demo Metrics
- [ ] Smooth 3-minute presentation
- [ ] Live transaction on block explorer
- [ ] Clear value proposition demonstrated
- [ ] Judges can understand the concept quickly

## Resources Needed

### Development Tools
- Foundry or Hardhat for smart contracts
- Node.js and Go environments
- Git for version control
- Testnet faucet access

### External Services
- Testnet RPC endpoint
- Block explorer for verification
- Optional: IPFS for image storage
- Optional: The Graph for indexing

### Team Roles (if applicable)
- **Smart Contract Developer**: Solidity implementation
- **Frontend Developer**: React/Next.js implementation
- **Backend Developer**: Go node agent and API
- **DevOps**: Deployment and testing

## Conclusion

This plan provides a comprehensive roadmap for building a compelling DePIN hackathon project that demonstrates real-world utility, technical innovation, and clear value proposition. The Edge AI Inference Marketplace showcases how blockchain technology can enable new economic models for distributed computing resources while maintaining security and user experience.

The modular architecture allows for incremental development and testing, ensuring a working demo even if not all features are completed within the time constraints.
