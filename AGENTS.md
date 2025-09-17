# AGENTS: Repository Guidance for AI Assistants

## Project Summary
- Edge AI Inference Marketplace monorepo with smart contracts, Next.js frontend, and Go node agent.

## Tech Stack
- Contracts: Solidity 0.8.x, Foundry.
- Frontend: Next.js 14, TypeScript, Wagmi + viem, Tailwind, shadcn/ui.
- Node Agent: Go 1.21+, Gin, go-ethereum, ONNX Runtime.

## Global Rules
- Prefer small, reviewable edits. Do not change unrelated files.
- Before editing, search for existing utils and patterns in the repo.
- After edits, run lints and tests for affected packages only.
- Keep security in mind: no plaintext secrets, validate inputs, avoid unsafe code.

## Coding Style
- TypeScript: strict mode, no any, explicit types on public APIs.
- Go: idiomatic Go, context plumbing, error wrapping with %w, no global state.
- Solidity: OZ libraries, checks-effects-interactions, custom errors, events for state changes.

## Testing Policy
- Contracts: unit tests for core flows, signature verification, failure cases.
- Frontend: component tests for critical flows, mock chain calls.
- Node agent: handler tests, signature unit tests, simple integration test.

## Operational Commands
- Contracts: build `forge build`, test `forge test -vvv`, deploy scripts in `scripts/`.
- Frontend: dev `pnpm --filter @apps/frontend dev`, build `pnpm -w build`.
- Node agent: run `go run cmd/server/main.go`, test `go test ./...`.

## AI Edit Guidance
- Ask for missing context (addresses, chainId) only if necessary.
- Don’t invent contract ABIs; import from `packages/contracts/out` once generated.
- Maintain `.cursorignore`; never index `node_modules`, build outputs, `.env*`, datasets.

## Security & Privacy
- Do not commit secrets. Use `.env.example` templates.
- Redact PII and keys from logs and prompts.

