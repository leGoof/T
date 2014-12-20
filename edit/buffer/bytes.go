// Package buffer provides unbounded, file-backed buffers.
package buffer

import (
	"io"
	"io/ioutil"
	"os"
	"strconv"
)

// A CountError records an error caused by a negative count.
type CountError int

func (err CountError) Error() string {
	return "invalid count: " + strconv.Itoa(int(err))
}

// A Bytes is an unbounded byte buffer backed by a file.
type Bytes struct {
	// F is the file that backs the buffer.
	// It is created lazily.
	f *os.File
	// BlockSize is the maximum number of bytes in a block.
	blockSize int
	// Blocks contains all blocks of the buffer in order.
	// Free contains blocks that are free to be re-allocated.
	blocks, free []block
	// End is the byte offset of the end of the backing file.
	end int64

	// Cache is the index of the block whose data is currently cached.
	cached int
	// Cache is the cached data.
	cache []byte
	// Dirty tracks whether the cached data has changed since it was read.
	dirty bool

	// Size is the size of the buffer.
	size int64
}

// A block describes a portion of the buffer and its location in the backing file.
type block struct {
	// Start is the byte offset of the block in the file.
	start int64
	// N is the number of bytes in the block.
	n int
}

// NewBytes returns a new, empty Bytes buffer.
// No more than blockSize bytes are cached in memory.
func NewBytes(blockSize int) *Bytes {
	return &Bytes{
		blockSize: blockSize,
		cached:    -1,
		cache:     make([]byte, blockSize),
	}
}

// Close closes the Bytes buffer and it's backing file.
func (b *Bytes) Close() error {
	b.cache = nil
	if b.f == nil {
		return nil
	}
	path := b.f.Name()
	if err := b.f.Close(); err != nil {
		return err
	}
	return os.Remove(path)
}

// Size returns the size of the Bytes buffer in bytes.
func (b *Bytes) Size() int64 {
	return b.size
}

// Read returns the bytes in the range of an Address in the buffer.
func (b *Bytes) Read(at Address) ([]byte, error) {
	if at.From < 0 || at.From > at.To || at.To > b.Size() {
		return nil, AddressError(at)
	}
	bs := make([]byte, at.Size())
	if _, err := b.ReadAt(bs, at.From); err != nil {
		return nil, err
	}
	return bs, nil
}

// Write writes bytes to the range of an Address in the buffer.
func (b *Bytes) Write(bs []byte, at Address) error {
	if at.From < 0 || at.From > at.To || at.To > b.Size() {
		return AddressError(at)
	}
	if _, err := b.delete(at.Size(), at.From); err != nil {
		return err
	}
	_, err := b.insert(bs, at.From)
	return err
}

// ReadAt reads bytes from the Bytes buffer starting at the address.
// The return value is the number of bytes read.
// If fewer than len(bs) bytes are read then the error states why.
// If the address is beyond the end of the buffer, 0 and io.EOF are returned.
func (b *Bytes) ReadAt(bs []byte, at int64) (int, error) {
	switch {
	case at < 0:
		return 0, AddressError(Point(at))
	case at == b.Size() && len(bs) == 0:
		return 0, nil
	case at >= b.Size():
		return 0, io.EOF
	}
	var tot int
	for len(bs) > 0 {
		if at == b.Size() {
			return tot, io.EOF
		}
		i, q0 := b.blockAt(at)
		blk, err := b.get(i)
		if err != nil {
			return tot, err
		}
		o := int(at - q0)
		m := copy(bs, b.cache[o:blk.n])
		bs = bs[m:]
		at += int64(m)
		tot += m
	}
	return tot, nil
}

// Insert adds the bytes to the address in the Bytes buffer.
// After adding, the byte at the address is the first of the added bytes.
// The return value is the number of bytes added and any error that was encountered.
// It is an error to add at a negative address or an address that is greater than the buffer size.
func (b *Bytes) insert(bs []byte, at int64) (int, error) {
	if at < 0 || at > b.Size() {
		return 0, AddressError(Point(at))
	}
	var tot int
	for len(bs) > 0 {
		i, q0 := b.blockAt(at)
		blk, err := b.get(i)
		if err != nil {
			return tot, err
		}
		m := b.blockSize - blk.n
		if m == 0 {
			if i, err = b.insertAt(at); err != nil {
				return tot, err
			}
			if blk, err = b.get(i); err != nil {
				return tot, err
			}
			q0 = at
			m = b.blockSize
		}
		if m > len(bs) {
			m = len(bs)
		}
		o := int(at - q0)
		copy(b.cache[o+m:], b.cache[o:blk.n])
		copy(b.cache[o:], bs[:m])
		b.dirty = true
		bs = bs[m:]
		blk.n += m
		b.size += int64(m)
		at += int64(m)
		tot += m
	}
	return tot, nil
}

// Delete deletes a range of bytes from the Bytes buffer.
// The return value is the number of bytes deleted.
// If fewer than n bytes are deleted, the error states why.
func (b *Bytes) delete(n, at int64) (int64, error) {
	if n < 0 {
		return 0, CountError(n)
	}
	if at < 0 || at+n > b.Size() {
		return 0, AddressError(Point(at))
	}
	var tot int64
	for n > 0 {
		i, q0 := b.blockAt(at)
		blk, err := b.get(i)
		if err != nil {
			return tot, err
		}
		o := int(at - q0)
		m := blk.n - o
		if int64(m) > n {
			m = int(n)
		}
		if o == 0 && n >= int64(blk.n) {
			// Remove the entire block.
			b.freeBlock(*blk)
			b.blocks = append(b.blocks[:i], b.blocks[i+1:]...)
			b.cached = -1
		} else {
			// Remove a portion of the block.
			copy(b.cache[o:], b.cache[o+m:])
			b.dirty = true
			blk.n -= m
		}
		n -= int64(m)
		tot += int64(m)
		b.size -= int64(m)
	}
	return tot, nil
}

func (b *Bytes) allocBlock() block {
	if l := len(b.free); l > 0 {
		blk := b.free[l-1]
		b.free = b.free[:l-1]
		return blk
	}
	blk := block{start: b.end}
	b.end += int64(b.blockSize)
	return blk
}

func (b *Bytes) freeBlock(blk block) {
	b.free = append(b.free, block{start: blk.start})
}

// BlockAt returns the index and start address of the block containing the address.
// If the address is one beyond the end of the file, a new block is allocated.
// BlockAt panics if the address is negative or more than one past the end.
func (b *Bytes) blockAt(at int64) (int, int64) {
	if at < 0 || at > b.Size() {
		panic(AddressError(Point(at)))
	}
	if at == b.Size() {
		i := len(b.blocks)
		blk := b.allocBlock()
		b.blocks = append(b.blocks[:i], append([]block{blk}, b.blocks[i:]...)...)
		return i, at
	}
	var q0 int64
	for i, blk := range b.blocks {
		if q0 <= at && at < q0+int64(blk.n) {
			return i, q0
		}
		q0 += int64(blk.n)
	}
	panic("impossible")
}

// insertAt inserts a block at the address and returns the new block's index.
// If a block contains the address then it is split.
func (b *Bytes) insertAt(at int64) (int, error) {
	i, q0 := b.blockAt(at)
	o := int(at - q0)
	blk := b.blocks[i]
	if at == q0 {
		// Adding immediately before blk, no need to split.
		nblk := b.allocBlock()
		b.blocks = append(b.blocks[:i], append([]block{nblk}, b.blocks[i:]...)...)
		if b.cached == i {
			b.cached = i + 1
		}
		return i, nil
	}

	// Splitting blk.
	// Make sure it's both on disk and in the cache.
	if b.cached == i && b.dirty {
		if err := b.put(); err != nil {
			return -1, err
		}
	} else if _, err := b.get(i); err != nil {
		return -1, err
	}

	// Resize blk.
	b.blocks[i].n = int(o)

	// Insert the new, empty block.
	nblk := b.allocBlock()
	b.blocks = append(b.blocks[:i+1], append([]block{nblk}, b.blocks[i+1:]...)...)

	// Allocate a block for the second half of blk and set it as the cache.
	// The next put will write it out.
	nblk = b.allocBlock()
	b.blocks = append(b.blocks[:i+2], append([]block{nblk}, b.blocks[i+2:]...)...)
	b.blocks[i+2].n = blk.n - o
	copy(b.cache, b.cache[o:])
	b.cached = i + 2
	b.dirty = true

	return i + 1, nil
}

// File returns an *os.File, creating a new file if one is not created yet.
func (b *Bytes) file() (*os.File, error) {
	if b.f == nil {
		f, err := ioutil.TempFile(os.TempDir(), "edit")
		if err != nil {
			return nil, err
		}
		b.f = f
	}
	return b.f, nil
}

// Put writes the cached block back to the file.
func (b *Bytes) put() error {
	if b.cached < 0 || !b.dirty || len(b.cache) == 0 {
		return nil
	}
	blk := b.blocks[b.cached]
	f, err := b.file()
	if err != nil {
		return err
	}
	if _, err := f.WriteAt(b.cache[:blk.n], blk.start); err != nil {
		return err
	}
	b.dirty = false
	return nil
}

// Get loads the cache with the data from the block at the given index,
// returning a pointer to it.
func (b *Bytes) get(i int) (*block, error) {
	if b.cached == i {
		return &b.blocks[i], nil
	}
	if err := b.put(); err != nil {
		return nil, err
	}

	blk := b.blocks[i]
	f, err := b.file()
	if err != nil {
		return nil, err
	}
	if _, err := f.ReadAt(b.cache[:blk.n], blk.start); err != nil {
		if err == io.EOF {
			panic("unexpected EOF")
		}
		return nil, err
	}
	b.cached = i
	b.dirty = false
	return &b.blocks[i], nil
}
