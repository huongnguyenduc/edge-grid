.PHONY: all build-wasm run-dev docker-up clean

# 1. Build file Wasm (Requires TinyGo)
build-wasm:
	tinygo build -o hello.wasm -target=wasi -scheduler=none examples/hello/main.go

# 2. Run Docker cluster
docker-up: build-wasm
	@echo "🚀 Launching Edge Grid Cluster..."
	docker-compose up --build

# 3. Clean up
clean:
	rm -rf data/
	docker-compose down
	rm hello.wasm