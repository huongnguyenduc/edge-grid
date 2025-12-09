package main

// 1. Create a static 4KB buffer to exchange data
var buffer [4096]byte

// 2. Export function to let Host know where this buffer is in RAM
//
//export getBufferPtr
func getBufferPtr() *byte {
	return &buffer[0]
}

// 3. Main processing function
// Input: length of data Host sent into buffer
// Output: length of data result Wasm wrote into buffer
//
//export handle
func handle(size uint32) uint32 {
	// A. Read data from buffer (Host wrote here)
	inputData := buffer[:size]
	inputStr := string(inputData)

	// B. Process (Reverse string)
	outputStr := reverse(inputStr)

	// C. Write result into buffer
	copy(buffer[:], outputStr)

	// D. Return new result length
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
