package fuse

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Maximum file write size we are prepared to receive from the kernel.
const maxWrite = 16 * 1024 * 1024

// All requests read from the kernel, without data, are shorter than
// this.
var maxRequestSize = syscall.Getpagesize()
var bufSize = maxRequestSize + maxWrite

type Buffers struct {
	// Raw []bytes
	inBytes  []byte
	outBytes []byte

	reqBytes  []byte
	respBytes []byte

	allocBytes []byte

	// Higher-level but still bytes
	reqMsg  message
	respBuf buffer

	alloc allocator
	scope RequestScope
}

func MakeBuffers() (r *Buffers) {
	r = new(Buffers)
	r.inBytes = make([]byte, bufSize)
	r.outBytes = make([]byte, bufSize)
	// This is way too many bytes for what we need
	r.reqBytes = make([]byte, bufSize)
	r.respBytes = make([]byte, bufSize)
	r.allocBytes = make([]byte, bufSize)
	r.alloc.init(r.allocBytes)
	r.scope.bufs = r
	return r
}

func (b *Buffers) Reset() *RequestScope {
	clear := b.inBytes[0:len(b.reqMsg.buf)]
	for i := range clear {
		clear[i] = 0
	}

	clear = b.outBytes[0:len(b.respBuf)]
	for i := range clear {
		clear[i] = 0
	}

	b.reqMsg.buf = b.inBytes[:]
	b.reqMsg.hdr = (*inHeader)(unsafe.Pointer(&b.inBytes[0]))
	newBuffer(&b.respBuf, b.outBytes)

	b.alloc.reset()

	b.scope.Req = nil
	b.scope.reqMsg = &b.reqMsg
	b.scope.respBuf = &b.respBuf
	b.scope.conn = nil
	b.scope.Alloc = &b.alloc
	b.scope.responded = false

	return &b.scope
}

type RequestScope struct {
	Req       Request
	reqMsg    *message
	respBuf   *buffer
	bufs      *Buffers
	conn      *Conn
	Alloc     *allocator
	responded bool
}

type Allocator interface {
	Alloc(size int) []byte
	Free(size int)
}

type allocator struct {
	buf  []byte
	next int
}

func (a *allocator) init(buf []byte) {
	a.buf = buf
	a.next = 0
}

func (a *allocator) Alloc(size int) []byte {
	s := int(size)
	if a.next+s > cap(a.buf) {
		panic(fmt.Sprintf("Not enough capacity: %v + %v > %v", a.next, size, cap(a.buf)))
	}
	r := a.buf[a.next : a.next+s]
	a.next += s
	return r
}

func (a *allocator) Free(size int) {
	a.next -= size
	if a.next < 0 {
		panic(fmt.Sprintf("allocator.next below 0: %v %v", a.next, size))
	}
}

func (a *allocator) reset() {
	b := a.buf[0:a.next]
	for idx := range b {
		b[idx] = 0
	}
	a.next = 0
}

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
