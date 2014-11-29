package edit

import (
	"testing"
	"unicode/utf8"
)

const testBlockSize = 8

// TestBufferBasics tests writing to a buffer and then reading it all back.
func TestBufferBasics(t *testing.T) {
	str := "Hello, World! ☺"
	l := int64(utf8.RuneCountInString(str))
	b := NewBuffer(testBlockSize)
	defer b.Close()

	err := b.Write([]rune(str), Address{})
	if err != nil {
		t.Fatalf(`Write(%s), Address{})=%v, want nil`, str, err)
	}
	if s := b.Size(); s != l {
		t.Fatalf(`Size()=%d, want %d`, s, l)
	}
	rs, err := b.Read(Address{0, b.Size()})
	if string(rs) != str || err != nil {
		t.Fatalf(`Read(Address{0, %d})="%v",%v, want %s,nil`, b.Size(), string(rs), err, str)
	}
}

func TestBufferWrite(t *testing.T) {
	tests := []struct {
		init  string
		write string
		at    Address
		want  string
		err   error
	}{
		{at: Address{From: 1, To: 2}, err: AddressError(Address{From: 1, To: 2})},
		{at: Address{From: -1, To: 0}, err: AddressError(Address{From: -1, To: 0})},
		{at: Address{From: 0, To: 1}, err: AddressError(Address{From: 0, To: 1})},
		{at: Address{From: 2, To: 1}, err: AddressError(Address{From: 2, To: 1})},

		{init: "", write: "", at: Address{}, want: ""},
		{init: "", write: "Hello, World!", at: Address{}, want: "Hello, World!"},
		{init: "", write: "Hello, 世界", at: Address{}, want: "Hello, 世界"},
		{init: "Hello, World!", write: "", at: Address{0, 13}, want: ""},
		{init: "Hello, !", write: "World", at: Address{7, 7}, want: "Hello, World!"},
		{init: "Hello, World", write: "! ☺", at: Address{12, 12}, want: "Hello, World! ☺"},
		{init: ", World!", write: "Hello", at: Address{0, 0}, want: "Hello, World!"},
	}
	for _, test := range tests {
		b := NewBuffer(testBlockSize)
		defer b.Close()
		if err := b.Write([]rune(test.init), Address{}); err != nil {
			t.Errorf("init Write(%v, Address{})=%v, want nil", test.init, err)
			continue
		}
		if err := b.Write([]rune(test.write), test.at); err != test.err {
			t.Errorf("Write(%v, %v)=%v, want %v", test.write, test.at, err, test.err)
			continue
		}
		if test.err != nil {
			continue
		}
		rs, err := b.Read(Address{0, b.Size()})
		if s := string(rs); s != test.want || err != nil {
			t.Errorf(`%+v Read(Address{0, %d})="%s",%v, want %v,nil`,
				test, b.Size(), s, err, test.want)
			continue
		}
	}
}

func TestBufferRead(t *testing.T) {
	tests := []struct {
		init string
		at   Address
		want string
		err  error
	}{
		{at: Address{From: 1, To: 2}, err: AddressError(Address{From: 1, To: 2})},
		{at: Address{From: -1, To: 0}, err: AddressError(Address{From: -1, To: 0})},
		{at: Address{From: 0, To: 1}, err: AddressError(Address{From: 0, To: 1})},
		{at: Address{From: 2, To: 1}, err: AddressError(Address{From: 2, To: 1})},

		{init: "", at: Address{}, want: ""},
		{init: "Hello, World!", at: Address{13, 13}, want: ""},
		{init: "Hello, World!", at: Address{0, 13}, want: "Hello, World!"},
		{init: "Hello, 世界", at: Address{0, 9}, want: "Hello, 世界"},
		{init: "Hello, 世界", at: Address{1, 9}, want: "ello, 世界"},
		{init: "Hello, 世界", at: Address{0, 8}, want: "Hello, 世"},
		{init: "Hello, 世界", at: Address{0, 8}, want: "Hello, 世"},
	}
	for _, test := range tests {
		b := NewBuffer(testBlockSize)
		defer b.Close()
		if err := b.Write([]rune(test.init), Address{}); err != nil {
			t.Errorf("init Write(%v, Address{})=%v, want nil", test.init, err)
			continue
		}
		rs, err := b.Read(test.at)
		if s := string(rs); s != test.want || err != test.err {
			t.Errorf(`Read(%v)="%s",%v, want %v,%v`,
				test.at, s, err, test.want, test.err)
			continue
		}
	}
}