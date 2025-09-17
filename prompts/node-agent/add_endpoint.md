Task: Add a new API endpoint to the Go node agent

Context:
- App: apps/node-agent (Gin)
- Ensure input validation and rate limiting
- Sign responses with EIP-712 where applicable

Instructions:
- Add handler in internal/handlers
- Wire route in cmd/server/main.go
- Add unit test and update README if needed
- Keep changes small and focused

Deliverables:
- Handler, route, and tests


