// Copyright © 2015, The T Authors.

package edit

import (
	"errors"
	"io"
	"sync"

	"github.com/eaburns/T/edit/runes"
)

// MaxRunes is the maximum number of runes to read into memory.
const MaxRunes = 4096

// A Buffer is an editable rune buffer.
type Buffer struct {
	lock     sync.RWMutex
	runes    *runes.Buffer
	eds      []*Editor
	seq, who int32
}

// NewBuffer returns a new, empty Buffer.
func NewBuffer() *Buffer {
	return newBuffer(runes.NewBuffer(1 << 12))
}

func newBuffer(rs *runes.Buffer) *Buffer { return &Buffer{runes: rs} }

// Close closes the Buffer.
// After Close is called, the Buffer is no longer editable.
func (buf *Buffer) Close() error {
	buf.lock.Lock()
	defer buf.lock.Unlock()
	return buf.runes.Close()
}

// Size returns the number of runes in the Buffer.
//
// This method must be called with the RLock held.
func (buf *Buffer) size() int64 { return buf.runes.Size() }

// Rune returns the ith rune in the Buffer.
//
// This method must be called with the RLock held.
func (buf *Buffer) rune(i int64) (rune, error) { return buf.runes.Rune(i) }

// Change changes the string identified by at
// to contain the runes from the Reader.
//
// This method must be called with the Lock held.
func (buf *Buffer) change(at addr, src runes.Reader) error {
	if err := buf.runes.Delete(at.size(), at.from); err != nil {
		return err
	}
	n, err := runes.Copy(buf.runes.Writer(at.from), src)
	if err != nil {
		return err
	}
	for _, ed := range buf.eds {
		for m := range ed.marks {
			ed.marks[m] = ed.marks[m].update(at, n)
		}
	}
	return nil
}

// An Editor edits a Buffer of runes.
type Editor struct {
	buf     *Buffer
	who     int32
	marks   map[rune]addr
	pending *log
}

// NewEditor returns an Editor that edits the given buffer.
func NewEditor(buf *Buffer) *Editor {
	buf.lock.Lock()
	defer buf.lock.Unlock()
	ed := &Editor{
		buf:     buf,
		who:     buf.who,
		marks:   make(map[rune]addr),
		pending: newLog(),
	}
	buf.who++
	buf.eds = append(buf.eds, ed)
	return ed
}

// Close closes the editor.
func (ed *Editor) Close() error {
	ed.buf.lock.Lock()
	defer ed.buf.lock.Unlock()

	eds := ed.buf.eds
	for i := range eds {
		if eds[i] == ed {
			ed.buf.eds = append(eds[:i], eds[:i+1]...)
			return ed.pending.close()
		}
	}
	return errors.New("already closed")
}

// Where returns rune offsets of the address.
func (ed *Editor) Where(a Address) (addr, error) {
	ed.buf.lock.RLock()
	defer ed.buf.lock.RUnlock()
	at, err := a.where(ed)
	if err != nil {
		return addr{}, err
	}
	return at, err
}

// Do performs an Edit on the Editor's Buffer.
func (ed *Editor) Do(e Edit, w io.Writer) error {
	return ed.do(func() (addr, error) { return e.do(ed, w) })
}

// Do applies changes to an Editor's Buffer.
//
// Changes are applied in two phases:
// Phase one logs the changes without modifying the Buffer.
// Phase two applies the changes to the Buffer.
// If the Buffer is modified between phases one and two,
// no changes are applied, and the proceedure restarts
// from phase one.
//
// The f function performs phase one.
// It is called with the Buffer's RLock held
// and the Editor's pending log cleared.
// f appends the desired changes to the Editor's pending log
// and returns the address over which they were computed.
// The returned address is used to compute and set dot
// after the change is applied.
// In the face of retries, f is called multiple times,
// so it must be idempotent.
func (ed *Editor) do(f func() (addr, error)) error {
	var marks map[rune]addr
	defer func() { ed.marks = marks }()
retry:
	marks = make(map[rune]addr, len(ed.marks))
	for r, a := range ed.marks {
		marks[r] = a
	}
	seq, at, err := pendChanges(ed, f)
	if err != nil {
		return err
	}
	if at, err = fixAddrs(at, ed.pending); err != nil {
		return err
	}
	switch retry, err := applyChanges(ed, seq); {
	case err != nil:
		return err
	case retry:
		goto retry
	}
	ed.marks['.'] = at
	marks = ed.marks
	return err
}

func pendChanges(ed *Editor, f func() (addr, error)) (int32, addr, error) {
	if err := ed.pending.clear(); err != nil {
		return 0, addr{}, err
	}

	ed.buf.lock.RLock()
	defer ed.buf.lock.RUnlock()
	seq := ed.buf.seq
	at, err := f()
	return seq, at, err
}

func applyChanges(ed *Editor, seq int32) (bool, error) {
	ed.buf.lock.Lock()
	defer ed.buf.lock.Unlock()
	if ed.buf.seq != seq {
		return true, nil
	}
	for e := logFirst(ed.pending); !e.end(); e = e.next() {
		if err := ed.buf.change(e.at, e.data()); err != nil {
			// TODO(eaburns): Very bad; what should we do?
			return false, err
		}
	}
	ed.buf.seq++
	return false, nil
}

func fixAddrs(at addr, l *log) (addr, error) {
	if !inSequence(l) {
		return addr{}, errors.New("changes not in sequence")
	}
	for e := logFirst(l); !e.end(); e = e.next() {
		if e.at.from == at.from {
			// If they have the same from, grow at.
			// This grows at, even if it's a point address,
			// to include the change made by e.
			// Otherwise, update would simply leave it
			// as a point address and move it.
			at.to = at.update(e.at, e.size).to
		} else {
			at = at.update(e.at, e.size)
		}
		for f := e.next(); !f.end(); f = f.next() {
			f.at = f.at.update(e.at, e.size)
			if err := f.store(); err != nil {
				return addr{}, err
			}
		}
	}
	return at, nil
}

func inSequence(l *log) bool {
	e := logFirst(l)
	for !e.end() {
		f := e.next()
		if f.at != e.at && f.at.from < e.at.to {
			return false
		}
		e = f
	}
	return true
}

func pend(ed *Editor, at addr, src runes.Reader) error {
	return ed.pending.append(ed.buf.seq, ed.who, at, src)
}

func (ed *Editor) lines(at addr) (l0, l1 int64, err error) {
	var i int64
	l0 = int64(1) // line numbers are 1 based.
	for ; i < at.from; i++ {
		r, err := ed.buf.rune(i)
		if err != nil {
			return 0, 0, err
		} else if r == '\n' {
			l0++
		}
	}
	l1 = l0
	for ; i < at.to; i++ {
		r, err := ed.buf.rune(i)
		if err != nil {
			return 0, 0, err
		} else if r == '\n' && i < at.to-1 {
			l1++
		}
	}
	return l0, l1, nil
}
