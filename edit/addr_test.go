// Copyright © 2015, The T Authors.

package edit

import (
	"errors"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/eaburns/T/edit/runes"
)

func TestDotAddress(t *testing.T) {
	str := "Hello, 世界!"
	sz := int64(utf8.RuneCountInString(str))
	tests := []addressTest{
		{text: str, dot: pt(0), addr: Dot, want: pt(0)},
		{text: str, dot: pt(5), addr: Dot, want: pt(5)},
		{text: str, dot: rng(5, 6), addr: Dot, want: rng(5, 6)},
		{text: str, dot: pt(sz), addr: Dot, want: pt(sz)},
		{text: str, dot: rng(0, sz), addr: Dot, want: rng(0, sz)},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestMarkAddress(t *testing.T) {
	str := "Hello, 世界!"
	tests := []addressTest{
		{text: str, marks: map[rune]addr{}, addr: Mark('☺'), err: "bad mark"},

		{text: str, marks: map[rune]addr{'a': {0, 0}}, addr: Mark('a'), want: pt(0)},
		{text: str, marks: map[rune]addr{}, addr: Mark('a'), want: pt(0)},
		{text: str, marks: map[rune]addr{'z': {0, 0}}, addr: Mark('z'), want: pt(0)},
		{text: str, marks: map[rune]addr{'z': {1, 9}}, addr: Mark('z'), want: rng(1, 9)},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestEndAddress(t *testing.T) {
	tests := []addressTest{
		{text: "", addr: End, want: pt(0)},
		{text: "Hello, World!", addr: End, want: pt(13)},
		{text: "Hello, 世界!", addr: End, want: pt(10)},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestRuneAddress(t *testing.T) {
	str := "Hello, 世界!"
	sz := int64(utf8.RuneCountInString(str))
	tests := []addressTest{
		{text: str, addr: Rune(0), want: pt(0)},
		{text: str, addr: Rune(3), want: pt(3)},
		{text: str, addr: Rune(sz), want: pt(sz)},

		{text: str, dot: pt(sz), addr: Rune(0), want: pt(sz)},
		{text: str, dot: pt(sz), addr: Rune(-3), want: pt(sz - 3)},
		{text: str, dot: pt(sz), addr: Rune(-sz), want: pt(0)},

		{text: str, addr: Rune(10000), err: "out of range"},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestLineAddress(t *testing.T) {
	tests := []addressTest{
		{text: "", addr: Line(0), want: pt(0)},
		{text: "aa", addr: Line(0), want: pt(0)},
		{text: "aa\n", addr: Line(0), want: pt(0)},
		{text: "aa", addr: Line(1), want: rng(0, 2)},
		{text: "aa\n", addr: Line(1), want: rng(0, 3)},
		{text: "\n", addr: Line(1), want: rng(0, 1)},
		{text: "", addr: Line(1), want: pt(0)},
		{text: "aa\nbb", addr: Line(2), want: rng(3, 5)},
		{text: "aa\nbb\n", addr: Line(2), want: rng(3, 6)},
		{text: "aa\n", addr: Line(2), want: pt(3)},
		{text: "aa\nbb\ncc", addr: Line(3), want: rng(6, 8)},
		{text: "aa\nbb\ncc\n", addr: Line(3), want: rng(6, 9)},
		{text: "aa\nbb\n", addr: Line(3), want: pt(6)},

		{dot: pt(2), text: "aa", addr: Line(0), want: pt(2)},
		{dot: pt(3), text: "aa\n", addr: Line(0), want: pt(3)},

		{text: "", addr: Line(2), err: "out of range"},
		{text: "aa", addr: Line(2), err: "out of range"},
		{text: "aa\n", addr: Line(3), err: "out of range"},
		{text: "aa\nbb", addr: Line(3), err: "out of range"},
		{text: "aa\nbb", addr: Line(10), err: "out of range"},

		{text: "", addr: Line(-1), want: pt(0)},
		{dot: pt(2), text: "aa", addr: Line(0).reverse(), want: rng(0, 2)},
		{dot: pt(3), text: "aa\n", addr: Line(0).reverse(), want: pt(3)},
		{dot: pt(2), text: "aa", addr: Line(-1), want: pt(0)},
		{dot: pt(1), text: "aa", addr: Line(-1), want: pt(0)},
		{dot: pt(1), text: "abc\ndef", addr: Line(-1), want: pt(0)},
		{dot: pt(3), text: "aa\n", addr: Line(-1), want: rng(0, 3)},
		{dot: pt(1), text: "\n", addr: Line(-1), want: rng(0, 1)},
		{dot: pt(5), text: "aa\nbb", addr: Line(-2), want: pt(0)},
		{dot: pt(6), text: "aa\nbb\n", addr: Line(-2), want: rng(0, 3)},
		{dot: pt(3), text: "aa\n", addr: Line(-2), want: pt(0)},
		{dot: pt(8), text: "aa\nbb\ncc", addr: Line(-3), want: pt(0)},
		{dot: pt(9), text: "aa\nbb\ncc\n", addr: Line(-3), want: rng(0, 3)},
		{dot: pt(6), text: "aa\nbb\n", addr: Line(-3), want: pt(0)},

		{text: "", addr: Line(-2), err: "out of range"},
		{dot: pt(2), text: "aa", addr: Line(-2), err: "out of range"},
		{dot: pt(3), text: "aa\n", addr: Line(-3), err: "out of range"},
		{dot: pt(5), text: "aa\nbb", addr: Line(-3), err: "out of range"},
		{dot: pt(5), text: "aa\nbb", addr: Line(-10), err: "out of range"},

		{text: "abc\ndef", dot: pt(1), addr: Line(0), want: rng(1, 4)},
		{text: "abc\ndef", dot: pt(4), addr: Line(1), want: rng(4, 7)},
		{text: "abc\ndef", dot: pt(3), addr: Line(-1), want: pt(0)},
		{text: "abc\ndef", dot: pt(4), addr: Line(-1), want: rng(0, 4)},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestRegexpAddress(t *testing.T) {
	tests := []addressTest{
		{text: "Hello, 世界!", addr: Regexp("/"), want: pt(0)},
		{text: "Hello, 世界!", addr: Regexp("/H"), want: rng(0, 1)},
		{text: "Hello, 世界!", addr: Regexp("/."), want: rng(0, 1)},
		{text: "Hello, 世界!", addr: Regexp("/世界"), want: rng(7, 9)},
		{text: "Hello, 世界!", addr: Regexp("/[^!]+"), want: rng(0, 9)},

		{text: "Hello, 世界!", dot: pt(10), addr: Regexp("?"), want: pt(10)},
		{text: "Hello, 世界!", dot: pt(10), addr: Regexp("?!"), want: rng(9, 10)},
		{text: "Hello, 世界!", dot: pt(10), addr: Regexp("?."), want: rng(9, 10)},
		{text: "Hello, 世界!", dot: pt(10), addr: Regexp("?H"), want: rng(0, 1)},
		{text: "Hello, 世界!", dot: pt(10), addr: Regexp("?[^!]+"), want: rng(0, 9)},

		{text: "Hello, 世界!", dot: pt(10), addr: Regexp("/H").reverse(), want: rng(0, 1)},
		{text: "Hello, 世界!", addr: Regexp("?H").reverse(), want: rng(0, 1)},

		// Wrap.
		{text: "Hello, 世界!", addr: Regexp("?世界"), want: rng(7, 9)},
		{text: "Hello, 世界!", dot: pt(8), addr: Regexp("/世界"), want: rng(7, 9)},

		{text: "", addr: Regexp("/()"), err: "operand"},
		{text: "Hello, 世界!", addr: Regexp("/☺"), err: "no match"},
		{text: "Hello, 世界!", addr: Regexp("?☺"), err: "no match"},
	}
	for _, test := range tests {
		test.run(t)
	}
}

// Tests regexp String().
func TestRegexpString(t *testing.T) {
	tests := []struct {
		re, want string
	}{
		{"", "//"},
		{"/", "//"},
		{"☺", "☺☺"},
		{"//", "//"},
		{"☺☺", "☺☺"},
		{`/\/`, `/\//`},
		{`☺\☺`, `☺\☺☺`},
		{"/abc", "/abc/"},
		{"/abc/", "/abc/"},
		{"☺abc", "☺abc☺"},
		{"☺abc☺", "☺abc☺"},
		{"/abc", "/abc/"},
		{`/abc\/`, `/abc\//`},
		{`☺abc\☺`, `☺abc\☺☺`},
	}
	for _, test := range tests {
		re := Regexp(test.re)
		if s := re.String(); s != test.want {
			t.Errorf("Regexp(%q).String()=%q, want %q", test.re, s, test.want)
		}
	}
}

func TestPlusAddress(t *testing.T) {
	tests := []addressTest{
		{text: "abc", addr: Line(0).Plus(Rune(3)), want: pt(3)},
		{text: "abc", addr: Rune(2).Plus(Rune(1)), want: pt(3)},
		{text: "abc", addr: Rune(2).Plus(Rune(-1)), want: pt(1)},
		{text: "abc\ndef", addr: Line(0).Plus(Line(1)), want: rng(0, 4)},
		{text: "abc\ndef", addr: Line(1).Plus(Line(1)), want: rng(4, 7)},
		{text: "abc\ndef", addr: Line(0).Plus(Line(-1)), want: pt(0)},
		{text: "abc\ndef", addr: Line(1).Plus(Line(-1)), want: rng(0, 4)},
		{text: "abc\ndef", addr: Rune(1).Plus(Line(0)), want: rng(1, 4)},

		{text: "abc\ndef", dot: pt(1), addr: Dot.Plus(Line(-1)), want: pt(0)},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestMinusAddress(t *testing.T) {
	tests := []addressTest{
		{text: "abc", addr: Line(0).Minus(Rune(0)), want: pt(0)},
		{text: "abc", addr: Rune(2).Minus(Rune(1)), want: pt(1)},
		{text: "abc", addr: Rune(2).Minus(Rune(-1)), want: pt(3)},
		{text: "abc\ndef", addr: Line(1).Minus(Line(1)), want: pt(0)},
		{text: "abc\ndef", dot: rng(1, 6), addr: Dot.Minus(Line(1)).Plus(Line(1)), want: rng(0, 4)},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestToAddress(t *testing.T) {
	tests := []addressTest{
		{text: "abc", addr: Line(0).To(End), want: rng(0, 3)},
		{text: "abc", dot: pt(1), addr: Dot.To(End), want: rng(1, 3)},
		{text: "abc\ndef", addr: Line(0).To(Line(1)), want: rng(0, 4)},
		{text: "abc\ndef", addr: Line(1).To(Line(2)), want: rng(0, 7)},
		{
			text: "abcabc",
			addr: Regexp("/abc").To(Regexp("/b")),
			want: rng(0, 2),
		},
		{
			text: "abc\ndef\nghi\njkl",
			dot:  pt(11),
			addr: Regexp("?abc?").Plus(Line(1)).To(Dot),
			want: rng(4, 11),
		},
		{text: "abc\ndef", addr: Line(0).To(Line(1)).To(Line(2)), want: rng(0, 7)},
	}
	for _, test := range tests {
		test.run(t)
	}
}

func TestThenAddress(t *testing.T) {
	tests := []addressTest{
		{text: "abcabc", addr: Regexp("/abc/").Then(Regexp("/b/")), want: rng(0, 5)},
		{text: "abcabc", addr: Regexp("/abc/").Then(Dot.Plus(Rune(1))), want: rng(0, 4)},
		{text: "abcabc", addr: Line(0).Plus(Rune(1)).Then(Dot.Plus(Rune(1))), want: rng(1, 2)},
		{text: "abcabc", addr: Line(0).To(Rune(1)).Then(Dot.Plus(Rune(1))), want: rng(0, 2)},
	}
	for _, test := range tests {
		test.run(t)
	}
}

type addressTest struct {
	text string
	// If rev==false, the match starts from 0.
	// If rev==true, the match starts from len(text).
	dot   addr
	marks map[rune]addr
	addr  Address
	want  addr
	err   string // regexp matching the error string
}

func (test addressTest) run(t *testing.T) {
	ed := NewEditor(NewBuffer())
	defer ed.buf.Close()
	if err := ed.buf.runes.Insert([]rune(test.text), 0); err != nil {
		t.Fatalf(`Put("%s")=%v, want nil`, test.text, err)
	}
	if test.marks != nil {
		ed.marks = test.marks
	}
	ed.marks['.'] = test.dot // Reset dot to the test dot.
	a, err := test.addr.whereFrom(test.dot.to, ed)
	var errStr string
	if err != nil {
		errStr = err.Error()
	}
	if a != test.want ||
		(test.err == "" && errStr != "") ||
		(test.err != "" && !regexp.MustCompile(test.err).MatchString(errStr)) {
		t.Errorf(`Address("%q").range(%d, %q)=%v, %v, want %v, %v`,
			test.addr.String(), test.dot, test.text, a, err,
			test.want, test.err)
	}
}

func pt(p int64) addr         { return addr{from: p, to: p} }
func rng(from, to int64) addr { return addr{from: from, to: to} }

func TestAddr(t *testing.T) {
	tests := []struct {
		a, left string
		want    Address
		err     string
	}{
		{a: "", want: nil},
		{a: " ", want: nil},
		{a: "\t\t", want: nil},
		{a: "\t\n\txyz", left: "\txyz", want: nil},
		{a: "\n#1", left: "#1", want: nil},

		{a: "#0", want: Rune(0)},
		{a: "#1", want: Rune(1)},
		{a: "#", want: Rune(1)},
		{a: "#12345", want: Rune(12345)},
		{a: "#12345xyz", left: "xyz", want: Rune(12345)},
		{a: " #12345xyz", left: "xyz", want: Rune(12345)},
		{a: " #1\t\n\txyz", left: "\txyz", want: Rune(1)},
		{a: "#" + strconv.Itoa(math.MaxInt64) + "0", err: "out of range"},

		{a: "0", want: Line(0)},
		{a: "1", want: Line(1)},
		{a: "12345", want: Line(12345)},
		{a: "12345xyz", left: "xyz", want: Line(12345)},
		{a: " 12345xyz", left: "xyz", want: Line(12345)},
		{a: " 1\t\n\txyz", left: "\txyz", want: Line(1)},
		{a: strconv.Itoa(math.MaxInt64) + "0", err: "out of range"},

		{a: "/", want: Regexp("/")},
		{a: "//", want: Regexp("//")},
		{a: "?", want: Regexp("?")},
		{a: "??", want: Regexp("??")},
		{a: "/abcdef", want: Regexp("/abcdef")},
		{a: "/abc/def", left: "def", want: Regexp("/abc/")},
		{a: "/abc def", want: Regexp("/abc def")},
		{a: "/abc def\nxyz", left: "xyz", want: Regexp("/abc def/")},
		{a: "?abcdef", want: Regexp("?abcdef")},
		{a: "?abc?def", left: "def", want: Regexp("?abc?")},
		{a: "?abc def", want: Regexp("?abc def")},
		{a: " ?abc def", want: Regexp("?abc def")},
		{a: "?abc def\nxyz", left: "xyz", want: Regexp("?abc def?")},
		{a: "/()", err: "operand"},

		{a: "$", want: End},
		{a: " $", want: End},
		{a: " $\t", want: End},

		{a: ".", want: Dot},
		{a: " .", want: Dot},
		{a: " .\t", want: Dot},

		{a: "'m", want: Mark('m')},
		{a: " 'z", want: Mark('z')},
		{a: " ' a", want: Mark('a')},
		{a: " ' a\t", want: Mark('a')},
		{a: "'\na", err: "bad mark"},
		{a: "'☺", err: "bad mark"},
		{a: "' ☺", err: "bad mark"},
		{a: "'", err: "bad mark"},

		{a: "+", want: Dot.Plus(Line(1))},
		{a: "+\n2", left: "2", want: Dot.Plus(Line(1))},
		{a: "+xyz", left: "xyz", want: Dot.Plus(Line(1))},
		{a: "+5", want: Dot.Plus(Line(5))},
		{a: "5+", want: Line(5).Plus(Line(1))},
		{a: "5+6", want: Line(5).Plus(Line(6))},
		{a: " 5 + 6", want: Line(5).Plus(Line(6))},
		{a: "-", want: Dot.Minus(Line(1))},
		{a: "-xyz", left: "xyz", want: Dot.Minus(Line(1))},
		{a: "-5", want: Dot.Minus(Line(5))},
		{a: "5-", want: Line(5).Minus(Line(1))},
		{a: "5-6", want: Line(5).Minus(Line(6))},
		{a: " 5 - 6", want: Line(5).Minus(Line(6))},
		{a: ".+#5", want: Dot.Plus(Rune(5))},
		{a: "$-#5", want: End.Minus(Rune(5))},
		{a: "$ - #5 + #3", want: End.Minus(Rune(5)).Plus(Rune(3))},
		{a: "+-", want: Dot.Plus(Line(1)).Minus(Line(1))},
		{a: " + - ", want: Dot.Plus(Line(1)).Minus(Line(1))},
		{a: " - + ", want: Dot.Minus(Line(1)).Plus(Line(1))},
		{a: "/abc/+++---", want: Regexp("/abc/").Plus(Line(1)).Plus(Line(1)).Plus(Line(1)).Minus(Line(1)).Minus(Line(1)).Minus(Line(1))},

		{a: ",", want: Line(0).To(End)},
		{a: ",xyz", left: "xyz", want: Line(0).To(End)},
		{a: " , ", want: Line(0).To(End)},
		{a: ",\n1", left: "1", want: Line(0).To(End)},
		{a: ",1", want: Line(0).To(Line(1))},
		{a: "1,", want: Line(1).To(End)},
		{a: "0,$", want: Line(0).To(End)},
		{a: ".,$", want: Dot.To(End)},
		{a: "1,2", want: Line(1).To(Line(2))},
		{a: " 1 , 2 ", want: Line(1).To(Line(2))},
		{a: ",-#5", want: Line(0).To(Dot.Minus(Rune(5)))},
		{a: " , - #5", want: Line(0).To(Dot.Minus(Rune(5)))},
		{a: ";", want: Line(0).Then(End)},
		{a: ";xyz", left: "xyz", want: Line(0).Then(End)},
		{a: " ; ", want: Line(0).Then(End)},
		{a: " ;\n1", left: "1", want: Line(0).Then(End)},
		{a: ";1", want: Line(0).Then(Line(1))},
		{a: "1;", want: Line(1).Then(End)},
		{a: "0;$", want: Line(0).Then(End)},
		{a: ".;$", want: Dot.Then(End)},
		{a: "1;2", want: Line(1).Then(Line(2))},
		{a: " 1 ; 2 ", want: Line(1).Then(Line(2))},
		{a: ";-#5", want: Line(0).Then(Dot.Minus(Rune(5)))},
		{a: " ; - #5 ", want: Line(0).Then(Dot.Minus(Rune(5)))},
		{a: ";,", want: Line(0).Then(Line(0).To(End))},
		{a: " ; , ", want: Line(0).Then(Line(0).To(End))},

		// Implicit +.
		{a: "1#2", want: Line(1).Plus(Rune(2))},
		{a: "#2 1", want: Rune(2).Plus(Line(1))},
		{a: "1/abc", want: Line(1).Plus(Regexp("/abc"))},
		{a: "/abc/1", want: Regexp("/abc/").Plus(Line(1))},
		{a: "?abc?1", want: Regexp("?abc?").Plus(Line(1))},
		{a: "$?abc", want: End.Plus(Regexp("?abc"))},
	}
	for _, test := range tests {
		a, left, err := Addr([]rune(test.a))
		ok := true
		if test.err != "" {
			ok = err != nil && regexp.MustCompile(test.err).MatchString(err.Error())
		} else {
			ok = err == nil && reflect.DeepEqual(a, test.want) && string(left) == string(test.left)
		}
		if !ok {
			t.Errorf(`Addr(%q)=%q,%q,%q, want %q,%q,%q`, test.a, a, left, err, test.want, test.left, test.err)
		}
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		a, b, want addr
		n          int64
	}{
		// b after a
		{a: addr{10, 20}, b: addr{25, 30}, n: 0, want: addr{10, 20}},
		{a: addr{10, 20}, b: addr{25, 30}, n: 100, want: addr{10, 20}},
		{a: addr{10, 20}, b: addr{20, 30}, n: 0, want: addr{10, 20}},
		{a: addr{10, 20}, b: addr{20, 30}, n: 100, want: addr{10, 20}},

		// b before a
		{a: addr{10, 20}, b: addr{0, 0}, n: 0, want: addr{10, 20}},
		{a: addr{10, 20}, b: addr{0, 0}, n: 100, want: addr{110, 120}},
		{a: addr{10, 20}, b: addr{0, 10}, n: 0, want: addr{0, 10}},
		{a: addr{10, 20}, b: addr{0, 10}, n: 100, want: addr{100, 110}},

		// b over the beginning of a
		{a: addr{10, 20}, b: addr{0, 15}, n: 0, want: addr{0, 5}},
		{a: addr{10, 20}, b: addr{0, 15}, n: 10, want: addr{10, 15}},
		{a: addr{0, 3}, b: addr{0, 1}, n: 7, want: addr{7, 9}},

		// b over the end of a
		{a: addr{10, 20}, b: addr{15, 21}, n: 0, want: addr{10, 15}},
		{a: addr{10, 20}, b: addr{15, 21}, n: 10, want: addr{10, 15}},

		// b within a
		{a: addr{10, 20}, b: addr{12, 18}, n: 0, want: addr{10, 14}},
		{a: addr{10, 20}, b: addr{12, 18}, n: 1, want: addr{10, 15}},
		{a: addr{10, 20}, b: addr{12, 18}, n: 100, want: addr{10, 114}},
		{a: addr{10, 20}, b: addr{15, 20}, n: 0, want: addr{10, 15}},
		{a: addr{10, 20}, b: addr{15, 20}, n: 10, want: addr{10, 25}},
		{a: addr{0, 19}, b: addr{18, 19}, n: 2, want: addr{0, 20}},

		// b over all of a
		{a: addr{10, 20}, b: addr{10, 20}, n: 0, want: addr{10, 10}},
		{a: addr{10, 20}, b: addr{0, 40}, n: 0, want: addr{0, 0}},
		{a: addr{10, 20}, b: addr{0, 40}, n: 100, want: addr{100, 100}},
	}
	for _, test := range tests {
		a := test.a.update(test.b, test.n)
		if a != test.want {
			t.Errorf("%v.update(%v, %d)=%v, want %v",
				test.a, test.b, test.n, a, test.want)
		}
	}
}

// TestAddressString tests that well-formed addresses
// have valid and parsable address Strings()s.
func TestAddressString(t *testing.T) {
	tests := []struct {
		addr, want Address // If want==nil, want is set to addr.
	}{
		{addr: Dot},
		{addr: End},
		{addr: All},
		{addr: Rune(0)},
		{addr: Rune(100)},
		// Rune(-100) is the string -#100, when parsed, the implicit . is inserted: .-#100.
		{addr: Rune(-100), want: Dot.Minus(Rune(100))},
		{addr: Line(0)},
		{addr: Line(100)},
		// Line(-100) is the string -100, when parsed, the implicit . is inserted: .-100.
		{addr: Line(-100), want: Dot.Minus(Line(100))},
		{addr: Mark('a')},
		{addr: Mark('z')},
		{addr: Regexp("/☺☹")},
		{addr: Regexp("/☺☹/")},
		{addr: Regexp("?☺☹")},
		{addr: Regexp("?☺☹?")},
		{addr: Dot.Plus(Line(1))},
		{addr: Dot.Minus(Line(1))},
		{addr: Dot.Minus(Line(1)).Plus(Line(1))},
		{addr: Rune(1).To(Rune(2))},
		{addr: Rune(1).Then(Rune(2))},
		{addr: Regexp("/func").Plus(Regexp(`/\(`))},
	}
	for _, test := range tests {
		if test.want == nil {
			test.want = test.addr
		}
		str := test.addr.String()
		got, _, err := Addr([]rune(str))
		if err != nil || got != test.want {
			t.Errorf("Addr(%q)=%v,%v want %q,nil", str, got.String(), err, test.want.String())
		}
	}
}

type errReaderAt struct{ error }

func (e *errReaderAt) ReadAt([]byte, int64) (int, error)      { return 0, e.error }
func (e *errReaderAt) WriteAt(b []byte, _ int64) (int, error) { return len(b), nil }

// TestIOErrors tests IO errors when computing addresses.
func TestIOErrors(t *testing.T) {
	rs := []rune("Hello,\nWorld!")
	tests := []string{
		"1",
		"#1+1",
		"$-1",
		"#3-1",
		"/World",
		"?World",
		".+/World",
		".-/World",
		"0,/World",
		"0;/World",
		"/Hello/+",
		"/Hello/-",
		"/Hello/,",
		"/Hello/;",
		"/Hello/,/World",
		"/Hello/;/World",
		"/Hello/+/World",
		"/Hello/-/World",
		"/Hello/,/World",
		"/Hello/;/World",
	}
	for _, test := range tests {
		addr, left, err := Addr([]rune(test))
		if err != nil || len(left) != 0 {
			t.Fatalf("Addr(%q)=%q,{},%v want _,{},nil", test, addr, err)
		}
		f := &errReaderAt{nil}
		r := runes.NewBufferReaderWriterAt(1, f)
		ed := NewEditor(newBuffer(r))
		defer ed.Close()

		if err := ed.buf.runes.Insert(rs, 0); err != nil {
			t.Fatalf("ed.buf.runes.Insert(%v, 0)=%v, want nil", strconv.Quote(string(rs)), err)
		}

		// All subsequent reads will be errors.
		f.error = errors.New("read error")
		if a, err := addr.where(ed); err != f.error {
			t.Errorf("Addr(%q).addr()=%v,%v, want addr{},%q", test, a, err, f.error)
		}
	}
}
