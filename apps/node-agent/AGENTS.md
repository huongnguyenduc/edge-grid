# AGENTS: Node Agent (Go) Guidance

## App Summary
- Go 1.21+ Gin HTTP server
- EIP-712 signing via `go-ethereum`
- Inference via ONNX Runtime (planned)

## Coding Rules
- Idiomatic Go; no global state
- Context plumbing on all handlers and outbound calls
- Error wrapping with `%w`; return typed errors where helpful
- Validate and bound all inputs; apply rate limiting

## API Conventions
- JSON request/response; explicit structs with json tags
- Validate inputs (size, types, ranges)
- Return `400` for client errors, `5xx` for server errors
- Health at `/health`; version at `/version`

## Security
- Never log private keys or PII
- Private key via env; use `go-ethereum` keystore or hex (dev only)
- Sign EIP-712 results deterministically; include nonce and deadline

## Testing
- Unit tests for handlers and signature utilities
- Integration test for happy path

## Commands
- Run: `go run cmd/server/main.go`
- Test: `go test ./...`
- Docker dev uses Air for hot reload

## Common Tasks
- Add handler in `internal/handlers`
- Wire route in `cmd/server/main.go`
- Update tests under `internal/...` or `cmd/...`

