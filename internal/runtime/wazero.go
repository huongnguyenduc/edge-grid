package runtime

import (
	"context"
	"fmt"
	"strings"

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

	// --- CẤU HÌNH QUAN TRỌNG NHẤT ---
	// WithStartFunctions(): Truyền rỗng để bảo Wazero ĐỪNG chạy hàm main()
	// Điều này giữ cho Module luôn MỞ (Open) để ta gọi hàm handle sau đó.
	modConfig := wazero.NewModuleConfig().WithStartFunctions()

	mod, err := w.runtime.InstantiateModule(ctx, code, modConfig)
	if err != nil {
		return nil, err
	}
	// Lưu ý: Đừng defer mod.Close(ctx) ở đây nếu bạn muốn dùng lại module,
	// nhưng trong kiến trúc Serverless function (mỗi request 1 lần chạy), ta close luôn cũng được.
	defer mod.Close(ctx)

	// 1. Lấy địa chỉ buffer
	ptrFunc := mod.ExportedFunction("getBufferPtr")
	if ptrFunc == nil {
		return nil, fmt.Errorf("function 'getBufferPtr' not found. Rebuild wasm?")
	}
	results, err := ptrFunc.Call(ctx)
	if err != nil {
		return nil, err
	}

	ptr := uint32(results[0])

	// 2. Ghi Input vào RAM
	if !mod.Memory().Write(ptr, input) {
		return nil, fmt.Errorf("memory write failed: out of bounds")
	}

	// 3. Gọi hàm handle
	handleFunc := mod.ExportedFunction("handle")
	if handleFunc == nil {
		return nil, fmt.Errorf("function 'handle' not found")
	}

	_, err = handleFunc.Call(ctx, uint64(len(input)))

	// Bắt lỗi Exit Code (Fallback an toàn)
	if err != nil {
		if strings.Contains(err.Error(), "exit_code(0)") {
			err = nil
		} else {
			return nil, fmt.Errorf("execution failed: %v", err)
		}
	}

	// 4. Đọc kết quả từ RAM
	bytes, ok := mod.Memory().Read(ptr, uint32(len(input)))
	if !ok {
		return nil, fmt.Errorf("memory read failed: out of bounds")
	}

	return bytes, nil
}

func (w *WasmRuntime) Close(ctx context.Context) {
	w.runtime.Close(ctx)
}
