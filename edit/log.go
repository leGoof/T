package edit

import "github.com/eaburns/T/runes"

// A log records edits made by an editor.
type log struct {
	runes *runes.Buffer
	n     int
}

type entry struct {
	header
	runes []rune
}

// HeaderRunes is the number of runes required to write a header.
const headerRunes = 7

type header struct {
	// Addr is the address after this edit was made.
	addr
	// Size0 is the initial size of the address before this edit.
	size0 int64
	// Seq is the sequence number of the edit.
	seq int32
}

// Push pushes an entry onto the log for address a changing to size n.
func (l *log) push(b *Buffer, a addr, n int64) error {
	rs := make([]rune, a.size())
	if _, err := b.runes.Read(rs, a.from); err != nil {
		return err
	}
	if _, err := l.runes.Insert(rs, l.runes.Size()); err != nil {
		return err
	}
	h := header{
		addr:  addr{a.from, a.from + n},
		size0: a.size(),
		seq:   b.seq,
	}
	if err := h.insert(l.runes, l.runes.Size()); err != nil {
		return err
	}
	l.n++
	return nil
}

func (l *log) top() (header, error) {
	from := l.runes.Size() - headerRunes
	if from < 0 {
		panic("bad log")
	}
	return readHeader(l.runes, l.runes.Size()-headerRunes)
}

func (l *log) pop() (entry, error) {
	var e entry
	var err error

	from := l.runes.Size() - headerRunes
	if from < 0 {
		panic("bad log")
	}
	e.header, err = readHeader(l.runes, l.runes.Size()-headerRunes)
	if err != nil {
		return entry{}, err
	}

	from -= e.size0
	if from < 0 {
		panic("bad log")
	}
	e.runes = make([]rune, e.size0)
	if _, err := l.runes.Read(e.runes, from); err != nil {
		return entry{}, err
	}

	if _, err := l.runes.Delete(l.runes.Size()-from, from); err != nil {
		return entry{}, err
	}
	l.n--
	return e, nil
}

// Insert inserts a header into the buffer.
func (h header) insert(b *runes.Buffer, to int64) error {
	var rs [headerRunes]rune
	rs[0] = rune(h.from & 0xFFFFFFFF)
	rs[1] = rune(h.from >> 32)
	rs[2] = rune(h.to & 0xFFFFFFFF)
	rs[3] = rune(h.to >> 32)
	rs[4] = rune(h.size0 & 0xFFFFFFFF)
	rs[5] = rune(h.size0 >> 32)
	rs[6] = h.seq
	_, err := b.Insert(rs[:], to)
	return err
}

// ReadHeader reads a header from a position in the buffer.
func readHeader(b *runes.Buffer, from int64) (header, error) {
	var rs [headerRunes]rune
	if _, err := b.Read(rs[:], from); err != nil {
		return header{}, err
	}
	var h header
	h.from = int64(rs[0]) | int64(rs[1])<<32
	h.to = int64(rs[2]) | int64(rs[3])<<32
	h.size0 = int64(rs[4]) | int64(rs[5])<<32
	h.seq = rs[6]
	return h, nil
}
