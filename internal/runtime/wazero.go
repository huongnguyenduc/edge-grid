package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type WasmRuntime struct {
	runtime wazero.Runtime
}

func NewWasmRuntime(ctx context.Context) (*WasmRuntime, error) {
	cfg := wazero.NewRuntimeConfigInterpreter()
	r := wazero.NewRuntimeWithConfig(ctx, cfg)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		return nil, fmt.Errorf("failed to instantiate WASI: %v", err)
	}

	return &WasmRuntime{runtime: r}, nil
}

func (w *WasmRuntime) Run(ctx context.Context, wasmBytes []byte, input []byte) ([]byte, error) {
	code, err := w.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, err
	}
	defer code.Close(ctx)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// --- MOST IMPORTANT CONFIGURATION ---
	// WithStartFunctions(): Pass empty to tell Wazero NOT to run main()
	// This keeps the Module always OPEN so we can call handle function later.
	modConfig := wazero.NewModuleConfig().WithStartFunctions()

	mod, err := w.runtime.InstantiateModule(ctx, code, modConfig)
	if err != nil {
		return nil, err
	}
	// Note: Don't defer mod.Close(ctx) here if you want to reuse the module,
	// but in Serverless function architecture (each request runs once), we can close it always.
	defer mod.Close(ctx)

	// 1. Get buffer address
	ptrFunc := mod.ExportedFunction("getBufferPtr")
	if ptrFunc == nil {
		return nil, fmt.Errorf("function 'getBufferPtr' not found. Rebuild wasm?")
	}
	results, err := ptrFunc.Call(ctx)
	if err != nil {
		return nil, err
	}

	ptr := uint32(results[0])

	// 2. Write Input to RAM
	if !mod.Memory().Write(ptr, input) {
		return nil, fmt.Errorf("memory write failed: out of bounds")
	}

	// 3. Call handle function
	handleFunc := mod.ExportedFunction("handle")
	if handleFunc == nil {
		return nil, fmt.Errorf("function 'handle' not found")
	}

	_, err = handleFunc.Call(ctx, uint64(len(input)))

	// Catch Exit Code error (Fallback to safety)
	if err != nil {
		if strings.Contains(err.Error(), "exit_code(0)") {
			err = nil
		}
		// Check Timeout Error
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return nil, fmt.Errorf("execution timed out (killed after 2s)")
		}
		return nil, fmt.Errorf("execution failed: %v", err)
	}

	// 4. Read result from RAM
	bytes, ok := mod.Memory().Read(ptr, uint32(len(input)))
	if !ok {
		return nil, fmt.Errorf("memory read failed: out of bounds")
	}

	return bytes, nil
}

func (w *WasmRuntime) Close(ctx context.Context) {
	w.runtime.Close(ctx)
}
