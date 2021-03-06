// Copyright © 2015, The T Authors.

package edit

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/eaburns/T/edit/runes"
	"github.com/eaburns/T/re1"
)

var (
	// All is the address of the entire buffer: 0,$.
	All Address = Line(0).To(End)
	// Dot is the address of the Editor's dot.
	Dot SimpleAddress = simpleAddr{dotAddr{}}
	// End is the address of the empty string at the end of the buffer.
	End SimpleAddress = simpleAddr{endAddr{}}
)

// An Address identifies a substring within a buffer.
type Address interface {
	// String returns the string representation of the address.
	// The returned string will result in an equivalent address
	// when parsed with Addr().
	String() string
	// To returns an address identifying the string
	// from the start of the receiver to the end of the argument.
	To(Address) Address
	// Then returns an address like To,
	// but with dot set to the receiver address
	// and with the argument evaluated from the end of the reciver
	Then(Address) Address
	// Plus returns an address identifying the string
	// of the argument address evaluated from the end of the receiver.
	Plus(SimpleAddress) Address
	// Minus returns an address identifying the string
	// of the argument address evaluated in reverse
	// from the start of the receiver.
	Minus(SimpleAddress) Address
	where(*Editor) (addr, error)
	whereFrom(from int64, ed *Editor) (addr, error)
}

// A addr identifies a substring within a buffer
// by its inclusive start offset and its exclusive end offset.
type addr struct{ from, to int64 }

// Size returns the number of runes in
// the string identified by the range.
func (a addr) size() int64 { return a.to - a.from }

// Update returns a, updated to account for b changing to size n.
func (a addr) update(b addr, n int64) addr {
	// Clip, unless b is entirely within a.
	if a.from >= b.from || b.to > a.to {
		if b.contains(a.from) {
			a.from = b.to
		}
		if b.contains(a.to - 1) {
			a.to = b.from
		}
		if a.from > a.to {
			a.to = a.from
		}
	}

	// Move.
	d := n - b.size()
	if a.to >= b.to {
		a.to += d
	}
	if a.from >= b.to {
		a.from += d
	}
	return a
}

func (a addr) contains(p int64) bool { return a.from <= p && p < a.to }

type compoundAddr struct {
	op     rune
	a1, a2 Address
}

func (a compoundAddr) To(a2 Address) Address {
	return compoundAddr{op: ',', a1: a, a2: a2}
}

func (a compoundAddr) Then(a2 Address) Address {
	return compoundAddr{op: ';', a1: a, a2: a2}
}

func (a compoundAddr) Plus(a2 SimpleAddress) Address {
	return addAddr{op: '+', a1: a, a2: a2}
}

func (a compoundAddr) Minus(a2 SimpleAddress) Address {
	return addAddr{op: '-', a1: a, a2: a2}
}

func (a compoundAddr) String() string {
	return a.a1.String() + string(a.op) + a.a2.String()
}

func (a compoundAddr) where(ed *Editor) (addr, error) {
	return a.whereFrom(0, ed)
}

func (a compoundAddr) whereFrom(from int64, ed *Editor) (addr, error) {
	a1, err := a.a1.whereFrom(from, ed)
	if err != nil {
		return addr{}, err
	}
	switch a.op {
	case ',':
		a2, err := a.a2.whereFrom(from, ed)
		if err != nil {
			return addr{}, err
		}
		return addr{from: a1.from, to: a2.to}, nil
	case ';':
		origDot := ed.marks['.']
		ed.marks['.'] = a1
		a2, err := a.a2.whereFrom(a1.to, ed)
		if err != nil {
			ed.marks['.'] = origDot // Restore dot on error.
			return addr{}, err
		}
		return addr{from: a1.from, to: a2.to}, nil
	default:
		panic("bad compound address")
	}
}

type addAddr struct {
	op rune
	a1 Address
	a2 SimpleAddress
}

func (a addAddr) To(a2 Address) Address {
	return compoundAddr{op: ',', a1: a, a2: a2}
}

func (a addAddr) Then(a2 Address) Address {
	return compoundAddr{op: ';', a1: a, a2: a2}
}

func (a addAddr) Plus(a2 SimpleAddress) Address {
	return addAddr{op: '+', a1: a, a2: a2}
}

func (a addAddr) Minus(a2 SimpleAddress) Address {
	return addAddr{op: '-', a1: a, a2: a2}
}

func (a addAddr) String() string {
	return a.a1.String() + string(a.op) + a.a2.String()
}

func (a addAddr) where(ed *Editor) (addr, error) {
	return a.whereFrom(0, ed)
}

func (a addAddr) whereFrom(from int64, ed *Editor) (addr, error) {
	a1, err := a.a1.whereFrom(from, ed)
	if err != nil {
		return addr{}, err
	}
	switch a.op {
	case '+':
		return a.a2.whereFrom(a1.to, ed)
	case '-':
		return a.a2.reverse().whereFrom(a1.from, ed)
	default:
		panic("bad additive address")
	}
}

// A SimpleAddress identifies a substring within a buffer.
// SimpleAddresses can be composed to form composite addresses.
type SimpleAddress interface {
	Address
	reverse() SimpleAddress
}

type simpAddrImpl interface {
	whereFrom(from int64, ed *Editor) (addr, error)
	String() string
	reverse() SimpleAddress
}

type simpleAddr struct {
	simpAddrImpl
}

func (a simpleAddr) To(a2 Address) Address {
	return compoundAddr{op: ',', a1: a, a2: a2}
}

func (a simpleAddr) Then(a2 Address) Address {
	return compoundAddr{op: ';', a1: a, a2: a2}
}

func (a simpleAddr) Plus(a2 SimpleAddress) Address {
	return addAddr{op: '+', a1: a, a2: a2}
}

func (a simpleAddr) Minus(a2 SimpleAddress) Address {
	return addAddr{op: '-', a1: a, a2: a2}
}

func (a simpleAddr) where(ed *Editor) (addr, error) {
	return a.whereFrom(0, ed)
}

type dotAddr struct{}

func (dotAddr) String() string { return "." }

func (dotAddr) whereFrom(_ int64, ed *Editor) (addr, error) {
	a := ed.marks['.']
	if a.from < 0 || a.to < a.from || a.to > ed.buf.size() {
		panic("bad dot")
	}
	return ed.marks['.'], nil
}

func (d dotAddr) reverse() SimpleAddress { return simpleAddr{d} }

type endAddr struct{}

func (endAddr) String() string { return "$" }

func (endAddr) whereFrom(_ int64, ed *Editor) (addr, error) {
	return addr{from: ed.buf.size(), to: ed.buf.size()}, nil
}

func (e endAddr) reverse() SimpleAddress { return simpleAddr{e} }

type markAddr rune

// Mark returns the address of the named mark rune.
// The rune must be a lower-case or upper-case letter or dot: [a-zA-Z.].
// An invalid mark rune results in an address that returns an error when evaluated.
func Mark(r rune) SimpleAddress { return simpleAddr{markAddr(r)} }

func (m markAddr) String() string { return "'" + string(rune(m)) }

func (m markAddr) whereFrom(_ int64, ed *Editor) (addr, error) {
	a := ed.marks[rune(m)]
	if a.from < 0 || a.to < a.from || a.to > ed.buf.size() {
		panic("bad mark")
	}
	if !isMarkRune(rune(m)) && m != '.' {
		return addr{}, errors.New("bad mark: " + string(rune(m)))
	}
	return a, nil
}

func (m markAddr) reverse() SimpleAddress { return simpleAddr{m} }

func isMarkRune(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

type runeAddr int64

// Rune returns the address of the empty string after rune n.
// If n is negative, this is equivalent to the compound address -#n.
func Rune(n int64) SimpleAddress { return simpleAddr{runeAddr(n)} }

func (n runeAddr) String() string {
	if n < 0 {
		return "-#" + strconv.FormatInt(int64(-n), 10)
	}
	return "#" + strconv.FormatInt(int64(n), 10)
}

func (n runeAddr) whereFrom(from int64, ed *Editor) (addr, error) {
	m := from + int64(n)
	if m < 0 || m > ed.buf.size() {
		return addr{}, errors.New("rune address out of range")
	}
	return addr{from: m, to: m}, nil
}

func (n runeAddr) reverse() SimpleAddress { return simpleAddr{runeAddr(-n)} }

type lineAddr struct {
	neg bool
	n   int
}

// Line returns the address of the nth full line.
// If n is negative, this is equivalent to the compound address -n.
func Line(n int) SimpleAddress {
	if n < 0 {
		return simpleAddr{lineAddr{neg: true, n: -n}}
	}
	return simpleAddr{lineAddr{n: n}}
}

func (l lineAddr) String() string {
	n := strconv.Itoa(int(l.n))
	if l.neg {
		return "-" + n
	}
	return n
}

func (l lineAddr) whereFrom(from int64, ed *Editor) (addr, error) {
	if l.neg {
		return l.rev(from, ed)
	}
	return l.fwd(from, ed)
}

func (l lineAddr) reverse() SimpleAddress {
	l.neg = !l.neg
	return simpleAddr{l}
}

func (l lineAddr) fwd(from int64, ed *Editor) (addr, error) {
	a := addr{from: from, to: from}
	if a.to > 0 {
		for a.to < ed.buf.size() {
			r, err := ed.buf.rune(a.to - 1)
			if err != nil {
				return a, err
			} else if r == '\n' {
				break
			}
			a.to++
		}
		if l.n > 0 {
			a.from = a.to
		}
	}
	for l.n > 0 && a.to < ed.buf.size() {
		r, err := ed.buf.rune(a.to)
		if err != nil {
			return a, err
		}
		a.to++
		if r == '\n' {
			l.n--
			if l.n > 0 {
				a.from = a.to
			}
		}
	}
	if l.n > 1 || l.n == 1 && a.to < ed.buf.size() {
		return addr{}, errors.New("line address out of range")
	}
	return a, nil
}

func (l lineAddr) rev(from int64, ed *Editor) (addr, error) {
	a := addr{from: from, to: from}
	if a.from < ed.buf.size() {
		for a.from > 0 {
			r, err := ed.buf.rune(a.from - 1)
			if err != nil {
				return a, err
			} else if r == '\n' {
				break
			}
			a.from--
		}
		a.to = a.from
	}
	for l.n > 0 && a.from > 0 {
		r, err := ed.buf.rune(a.from - 1)
		if err != nil {
			return a, err
		}
		a.from--
		if r == '\n' {
			l.n--
			a.to = a.from + 1
		} else if a.from == 0 {
			a.to = a.from
		}
	}
	if l.n > 1 {
		return addr{}, errors.New("line address out of range")
	}
	for a.from > 0 {
		r, err := ed.buf.rune(a.from - 1)
		if err != nil {
			return a, err
		} else if r == '\n' {
			break
		}
		a.from--
	}
	return a, nil
}

// ErrNoMatch is returned when a regular expression address fails to match.
var ErrNoMatch = errors.New("no match")

type reAddr struct {
	rev bool
	re  string
}

// Regexp returns an address identifying the next match of a regular expression.
// The regular expression must be a delimited regular expression
// using the syntax of the re1 package:
// http://godoc.org/github.com/eaburns/T/re1.
// If the delimiter is a ? then the regular expression is matched in reverse.
// The regular expression is not compiled until the address is computed
// on a buffer, so compilation errors will not be returned until that time.
func Regexp(re string) SimpleAddress {
	if len(re) == 0 {
		re = "/"
	}
	return simpleAddr{reAddr{rev: re[0] == '?', re: withTrailingDelim(re)}}
}

func withTrailingDelim(re string) string {
	var esc bool
	var rs []rune
	d, _ := utf8.DecodeRuneInString(re)
	for i, ru := range re {
		rs = append(rs, ru)
		// Ensure an unescaped trailing delimiter.
		if i == len(re)-utf8.RuneLen(ru) && (i == 0 || ru != d || esc) {
			rs = append(rs, d)
		}
		esc = !esc && ru == '\\'
	}
	return string(rs)
}

func (r reAddr) String() string { return r.re }

type forward struct {
	*runes.Buffer
	err error
}

func (rs *forward) Rune(i int64) rune {
	if rs.err != nil {
		return -1
	}
	r, err := rs.Buffer.Rune(i)
	if err != nil {
		rs.err = err
		return -1
	}
	return r
}

type reverse struct{ *forward }

func (rs *reverse) Rune(i int64) rune {
	return rs.forward.Rune(rs.Size() - i - 1)
}

func (r reAddr) whereFrom(from int64, ed *Editor) (a addr, err error) {
	re, err := re1.Compile([]rune(r.re), re1.Options{Delimited: true, Reverse: r.rev})
	if err != nil {
		return a, err
	}
	fwd := &forward{Buffer: ed.buf.runes}
	rs := re1.Runes(fwd)
	if r.rev {
		rs = &reverse{fwd}
		from = rs.Size() - from
	}
	switch match := re.Match(rs, from); {
	case fwd.err != nil:
		return a, fwd.err
	case match == nil:
		return a, ErrNoMatch
	default:
		a = addr{from: match[0][0], to: match[0][1]}
		if r.rev {
			a.from, a.to = rs.Size()-a.to, rs.Size()-a.from
		}
		return a, nil
	}

}

func (r reAddr) reverse() SimpleAddress {
	r.rev = !r.rev
	return simpleAddr{r}
}

const (
	digits        = "0123456789"
	simpleFirst   = "#/?$.'" + digits
	additiveFirst = "+-" + simpleFirst
)

// Addr parses and returns an address and the remaining runes.
// Addresses are terminated by a newline or end of input.
//
// The address syntax for address a0 is:
//	a0:	{a0} ',' {a0} | {a0} ';' {a0} | {a0} '+' {a1} | {a0} '-' {a1} | a0 a1 | a1
//	a1:	'$' | '.'| '\'' l | '#'{n} | n | '/' regexp {'/'} | '?' regexp {'?'}
//	n:	[0-9]+
//	l:	[a-z]
//	regexp:	<a valid re1 regular expression>
// All address operators are left-associative.
// The '+' and '-' operators are higher-precedence than ',' and ';'.
//
// Production a1 describes simple addresses:
//	$ is the empty string at the end of the buffer.
//	. is the current address of the editor, called dot.
//	'l is the address of the mark named l, where l is a lower-case or upper-case letter: [a-zA-Z.]
//	#{n} is the empty string after rune number n. If n is missing then 1 is used.
//	n is the nth line in the buffer. 0 is the string before the first full line.
//	'/' regexp {'/'} is the first match of the regular expression.
//	'?' regexp {'?'} is the first match of the regular expression going in reverse.
//
// Production a0 describes compound addresses:
//	{a0} ',' {a0} is the string from the start of the first address to the end of the second.
//		If the first address is missing, 0 is used.
//		If the second address is missing, $ is used.
//	{a0} ';' {a0} is like the previous,
//		but with the second address evaluated from the end of the first
//		with dot set to the first address.
//		If the first address is missing, 0 is used.
//		If the second address is missing, $ is used.
//	{a0} '+' {a0} is the second address evaluated from the end of the first.
//		If the first address is missing, . is used.
//		If the second address is missing, 1 is used.
//	{a0} '-' {a0} is the second address evaluated in reverse from the start of the first.
//		If the first address is missing, . is used.
//		If the second address is missing, 1 is used.
// If two addresses of the form a0 a1 are present and distinct then a '+' is inserted, as in a0 '+' a1.
func Addr(rs []rune) (Address, []rune, error) {
	a, rs, err := parseCompoundAddr(rs)
	if len(rs) > 0 && rs[0] == '\n' {
		// Trim the terminating newline.
		rs = rs[1:]
	}
	return a, rs, err
}

func parseCompoundAddr(rs []rune) (Address, []rune, error) {
	var a1 Address
	for {
		if len(rs) == 0 {
			return a1, rs, nil
		}
		var err error
		switch r := rs[0]; {
		case strings.ContainsRune(simpleFirst, r):
			var a2 SimpleAddress
			switch a2, rs, err = parseSimpleAddr(rs); {
			case err != nil:
				return nil, rs, err
			case a1 != nil:
				a1 = a1.Plus(a2)
			default:
				a1 = a2
			}
		case r == '+' || r == '-':
			if a1 == nil {
				a1 = Dot
			}
			var err error
			var a2 SimpleAddress
			a2, rs, err = parseSimpleAddr(rs[1:])
			switch {
			case err != nil:
				return nil, rs, err
			case a2 == nil:
				a2 = Line(1)
			}
			if r == '+' {
				a1 = a1.Plus(a2)
			} else {
				a1 = a1.Minus(a2)
			}
		case r == ',' || r == ';':
			if a1 == nil {
				a1 = Line(0)
			}
			var err error
			var a2 Address
			a2, rs, err = parseCompoundAddr(rs[1:])
			switch {
			case err != nil:
				return nil, rs, err
			case a2 == nil:
				a2 = End
			}
			if r == ',' {
				a1 = a1.To(a2)
			} else {
				a1 = a1.Then(a2)
			}
		case unicode.IsSpace(r) && r != '\n':
			rs = rs[1:]
		default:
			return a1, rs, nil
		}
	}
}

func parseSimpleAddr(rs []rune) (SimpleAddress, []rune, error) {
	for len(rs) > 0 {
		if len(rs) == 0 {
			return nil, rs, nil
		}
		var err error
		var a SimpleAddress
		switch r := rs[0]; {
		case r == '\'':
			a, rs, err = parseMarkAddr(rs)
		case r == '#':
			a, rs, err = parseRuneAddr(rs)
		case strings.ContainsRune(digits, r):
			a, rs, err = parseLineAddr(rs)
		case r == '/' || r == '?':
			var exp []rune
			if exp, rs, err = parseRegexp(rs); err != nil {
				return nil, rs, err
			}
			return Regexp(string(exp)), rs, nil
		case r == '$':
			a = End
			rs = rs[1:]
		case r == '.':
			a = Dot
			rs = rs[1:]
		case unicode.IsSpace(r) && r != '\n':
			rs = rs[1:]
		default:
			return nil, rs, nil
		}
		if a != nil || err != nil {
			return a, rs, err
		}
	}
	return nil, rs, nil
}

func parseMarkAddr(rs []rune) (SimpleAddress, []rune, error) {
	if rs[0] != '\'' {
		panic("not a mark address")
	}
	n := 1
	for ; n < len(rs) && unicode.IsSpace(rs[n]) && rs[n] != '\n'; n++ {
	}
	if n >= len(rs) || !isMarkRune(rs[n]) {
		got := "EOF"
		if n < len(rs) {
			got = string(rs[n])
		}
		return nil, rs[n:], errors.New("bad mark: " + got)
	}
	return Mark(rs[n]), rs[n+1:], nil
}

func parseRuneAddr(rs []rune) (SimpleAddress, []rune, error) {
	if rs[0] != '#' {
		panic("not a rune address")
	}
	var n int
	for n = 1; n < len(rs) && strings.ContainsRune(digits, rs[n]); n++ {
	}
	s := "1"
	if n > 1 {
		s = string(rs[1:n])
	}
	const base, bits = 10, 64
	r, err := strconv.ParseInt(s, base, bits)
	return Rune(r), rs[n:], err
}

func parseLineAddr(rs []rune) (SimpleAddress, []rune, error) {
	if !strings.ContainsRune(digits, rs[0]) {
		panic("not a line address")
	}
	var n int
	for n = 1; n < len(rs) && strings.ContainsRune(digits, rs[n]); n++ {
	}
	l, err := strconv.Atoi(string(rs[:n]))
	return Line(l), rs[n:], err
}
