// Package runes provides unbounded, file-backed rune buffers
// and io-package-style interfaces for reading and writing rune slices.
package runes

import (
	"bytes"
	"io"
)

// MinRead is the minimum rune buffer size
// passed to a Read call.
const MinRead = bytes.MinRead

// Reader wraps the basic Read method.
// It behaves like io.Reader
// but it accepts a slice of runes
// instead of a slice of bytes.
type Reader interface {
	Read([]rune) (int, error)
}

// WriterTo wraps the WriteTo method.
// It writes runes to the writer
// until there are no more runes to write.
type WriterTo interface {
	WriteTo(Writer) (int64, error)
}

// Writer wraps the basic Write method.
// It behaves like io.Writer
// but it accepts a slice of runes
// instead of a slice of bytes.
type Writer interface {
	Write([]rune) (int, error)
}

// ReaderFrom wraps the ReadFrom method.
// It reads runes from the reader
// until there are no more runes to read.
type ReaderFrom interface {
	ReadFrom(Reader) (int64, error)
}

// LimitedReader wraps a Reader,
// limiting the number of runes read.
// When the limit is reached, io.EOF is returned.
type LimitedReader struct {
	Reader
	// N is the the number of bytes to read.
	// It should not be changed after calling Read.
	N int64
	n int64
}

// Size returns the maximum number of runes
// remaining to be read.
// This is an upper bound.
// If the underlying Reader has fewer runes
// LimitedReader will read fewer runes.
func (r *LimitedReader) Size() int64 { return r.N - r.n }

func (r *LimitedReader) Read(p []rune) (int, error) {
	if r.n >= r.N {
		return 0, io.EOF
	}
	n := len(p)
	if max := r.Size(); max < int64(n) {
		n = int(max)
	}
	m, err := r.Reader.Read(p[:n])
	r.n += int64(m)
	return m, err
}

// A SliceReader is a Reader
// that reads from a rune slice.
type SliceReader struct {
	rs []rune
}

func (r *SliceReader) Read(p []rune) (int, error) {
	if len(r.rs) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.rs)
	r.rs = r.rs[n:]
	return n, nil
}

// Size returns the number of runes
// remaining to be read.
func (r SliceReader) Size() int64 { return int64(len(r.rs)) }

// ReadAll reads runes from the reader
// until an error or io.EOF is encountered.
// It returns all of the runes read.
// On success, the error is nil, not io.EOF.
func ReadAll(r Reader) ([]rune, error) {
	var rs []rune
	p := make([]rune, MinRead)
	for {
		n, err := r.Read(p)
		rs = append(rs, p[:n]...)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return rs, err
		}
	}
}
