# Environment Reference

This document lists environment variables used across the monorepo.

## Root
- `CHAIN_ID` (number): default chain id (e.g., 11155111)
- `RPC_URL` (string): RPC endpoint
- `CONTRACT_ADDRESS` (address): deployed JobEscrow (if any)

## Frontend (`apps/frontend`)
- `NEXT_PUBLIC_CHAIN_ID` (number)
- `NEXT_PUBLIC_CONTRACT_ADDRESS` (address)
- `NEXT_PUBLIC_NODE_AGENT_URL` (url) – default `http://localhost:8080`

## Node Agent (`apps/node-agent`)
- `PORT` (number) – default 8080
- `CHAIN_ID` (number)
- `PRIVATE_KEY` (hex) – dev only; do not use real keys
- `CONTRACT_ADDRESS` (address)

Example files to create locally:

Root `.env`:
```
CHAIN_ID=11155111
RPC_URL=https://sepolia.infura.io/v3/YOUR_PROJECT_ID
CONTRACT_ADDRESS=0xYourContractAddress
```

Frontend `.env.local`:
```
NEXT_PUBLIC_CHAIN_ID=11155111
NEXT_PUBLIC_CONTRACT_ADDRESS=0xYourContractAddress
NEXT_PUBLIC_NODE_AGENT_URL=http://localhost:8080
```

Node Agent `.env`:
```
PORT=8080
CHAIN_ID=11155111
PRIVATE_KEY=0x0000000000000000000000000000000000000000000000000000000000000001
CONTRACT_ADDRESS=0xYourContractAddress
```
