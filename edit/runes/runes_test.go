package runes

import (
	"io"
	"reflect"
	"testing"
	"unicode/utf8"
)

var (
	helloWorldTestRunes = []rune("Hello, World! αβξ")
	helloWorldReadTests = readTests{
		{0, ""},
		{5, "Hello"},
		{2, ", "},
		{6, "World!"},
		{0, ""},
		{1, " "},
		{100, "αβξ"},
	}
)

func TestSliceReader(t *testing.T) {
	r := &SliceReader{helloWorldTestRunes}
	left := int64(len(helloWorldTestRunes))
	helloWorldReadTests.runWithSize(t, r, left)
}

// TestLimitedReaderBigReader tests the LimitedReader
// where the underlying reader is bigger than the limit.
func TestLimitedReaderBigReader(t *testing.T) {
	left := int64(len(helloWorldTestRunes))
	bigRunes := make([]rune, left*10)
	copy(bigRunes, helloWorldTestRunes)
	r := &LimitedReader{Reader: &SliceReader{bigRunes}, N: left}
	helloWorldReadTests.runWithSize(t, r, left)
}

// TestLimitedReaderSmallReader tests the LimitedReader
// where the underlying reader is smaller than the limit.
func TestLimitedReaderSmallReader(t *testing.T) {
	// Chop off the last 3 runes,
	// and the last readTest element.
	rs := helloWorldTestRunes[:len(helloWorldTestRunes)-3]
	tests := helloWorldReadTests[:len(helloWorldReadTests)-1]

	left := int64(len(helloWorldTestRunes))
	r := &LimitedReader{Reader: &SliceReader{rs}, N: left}
	tests.runWithSize(t, r, left)
}

type readTests []struct {
	n    int
	want string
}

type readerSize interface {
	Reader
	Size() int64
}

func (tests readTests) runWithSize(t *testing.T, r readerSize, left int64) {
	for _, test := range tests {
		if n := r.Size(); n != left {
			t.Errorf("r.Size()=%d, want %d", n, left)
			return
		}
		if !readOK(t, r, test.n, test.want) {
			return
		}
		left -= int64(utf8.RuneCountInString(test.want))
	}
	n, err := r.Read(make([]rune, 1))
	if n != 0 || err != io.EOF {
		t.Errorf("Read(len=1)=%d,%v, want 0,io.EOF", n, err)
	}
}

func readOK(t *testing.T, r Reader, n int, want string) bool {
	w := []rune(want)
	p := make([]rune, n)
	m, err := r.Read(p)
	if m != len(w) || !reflect.DeepEqual(p[:m], w) || (err != nil && err != io.EOF) {
		t.Errorf("Read(len=%d)=%d,%v; %q want %d,<nil>; %q",
			n, m, err, string(p[:m]), len(w), want)
		return false
	}
	return true
}
