//go:build !cannon64
// +build !cannon64

package memory

func NewMemory() *Memory {
	return NewBinaryTreeMemory()
}
