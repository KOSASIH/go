package heap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeapPushAndPop(t *testing.T) {
	less := func(a, b int) bool { return a < b }
	h := New(less, 0)

	h.Push(3)
	h.Push(1)
	h.Push(2)
	h.Push(1)

	assert.Equal(t, 1, h.Pop())
	assert.Equal(t, 1, h.Pop())
	assert.Equal(t, 2, h.Pop())
	assert.Equal(t, 3, h.Pop())
}

func TestHeapPeek(t *testing.T) {
	less := func(a, b int) bool { return a < b }
	h := New(less, 0)

	h.Push(3)
	h.Push(1)
	h.Push(2)

	assert.Equal(t, 1, h.Peek())
	assert.Equal(t, 1, h.Pop())
}

func TestHeapLen(t *testing.T) {
	less := func(a, b int) bool { return a < b }
	h := New(less, 0)

	assert.Equal(t, 0, h.Len())
	h.Push(5)
	assert.Equal(t, 1, h.Len())
	h.Push(6)
	h.Push(7)
	assert.Equal(t, 3, h.Len())

	h.Pop()
	assert.Equal(t, 2, h.Len())
}
