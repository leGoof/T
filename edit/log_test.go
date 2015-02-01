package edit

import (
	"reflect"
	"testing"

	"github.com/eaburns/T/runes"
)

func TestLogPushPop(t *testing.T) {
	str := []rune("Hello, 世界!")
	sz := int64(len(str))
	b := NewBuffer()
	if _, err := b.runes.Insert(str, 0); err != nil {
		t.Fatalf("failed to initialize buffer: %v", err)
	}

	l := &log{runes: runes.NewBuffer(testBlockSize)}

	testPush(t, l, b, addr{0, 10}, 0, 1)
	testPush(t, l, b, addr{5, 5}, 10, 2)
	testPush(t, l, b, addr{0, 10}, 20, 3)

	e2 := entry{
		header: header{addr: addr{0, 20}, size0: sz, seq: 2},
		runes:  []rune(str),
	}
	testPop(t, l, e2, 2)

	e1 := entry{
		header: header{addr: addr{5, 15}, size0: 0, seq: 1},
		runes:  []rune{},
	}
	testPop(t, l, e1, 1)

	e0 := entry{
		header: header{addr: addr{0, 0}, size0: sz, seq: 0},
		runes:  []rune(str),
	}
	testPop(t, l, e0, 0)
}

func testPush(t *testing.T, l *log, b *Buffer, a addr, sz int64, n int) {
	if err := l.push(b, a, sz); err != nil {
		t.Fatalf("l.push(b, %v, %v)=%v, want nil", a, sz, err)
	}
	if l.n != n {
		t.Fatalf("l.n=%d, want %d", l.n, n)
	}
	b.seq++
}

func testPop(t *testing.T, l *log, want entry, n int) {
	got, err := l.pop()
	if !reflect.DeepEqual(got, want) || err != nil {
		t.Fatalf("l.pop()=%+v,%v, want %+v,nil", got, err, want)
	}
	if l.n != n {
		t.Fatalf("l.n=%d, want %d", l.n, n)
	}
}
