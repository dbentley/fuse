package fuse

import (
	"fmt"
	"unsafe"
)

// buffer provides a mechanism to write out a message in segments. This is useful because
// the fuse protocol means we have to assemble one contiguous byte array with several headers
// and then data.
type buffer []byte

func newBuffer(buf *buffer, bytes []byte) {
	*buf = bytes[0:0]
}

// newSegment creates a new segment of newSize bytes and returns a pointer to the new segment.
func (w *buffer) newSegment(newSize uintptr) unsafe.Pointer {
	return unsafe.Pointer(&(w.newSlice(newSize)[0]))
}

func (w *buffer) newSlice(newSize uintptr) []byte {
	s := int(newSize)
	if len(*w)+s > cap(*w) {
		panic(fmt.Sprintf("Not enough capacity: %v + %v > %v", len(*w), newSize, cap(*w)))
	}
	l := len(*w)
	*w = (*w)[:l+s]
	return (*w)[l : l+s]
}

func (w *buffer) truncate(extra int) {
	l := len(*w)
	clear := (*w)[l-extra:]
	for i := range clear {
		clear[i] = 0
	}

	*w = (*w)[:l-extra]
}
