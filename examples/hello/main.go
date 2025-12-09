package main

// 1. Tạo một bộ đệm tĩnh 4KB để trao đổi dữ liệu
var buffer [4096]byte

// 2. Export hàm để Host biết địa chỉ bộ đệm này nằm đâu trong RAM
//
//export getBufferPtr
func getBufferPtr() *byte {
	return &buffer[0]
}

// 3. Hàm xử lý chính
// Input: độ dài dữ liệu Host đã gửi vào buffer
// Output: độ dài dữ liệu kết quả Wasm đã ghi đè vào buffer
//
//export handle
func handle(size uint32) uint32 {
	// A. Đọc dữ liệu từ buffer (Host đã ghi vào đây)
	inputData := buffer[:size]
	inputStr := string(inputData)

	// B. Xử lý (Đảo ngược chuỗi)
	outputStr := reverse(inputStr)

	// C. Ghi đè kết quả vào buffer
	copy(buffer[:], outputStr)

	// D. Trả về độ dài kết quả mới
	return uint32(len(outputStr))
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func main() {}
