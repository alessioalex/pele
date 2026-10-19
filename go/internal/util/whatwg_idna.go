package util

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/binary"
	"io"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"
)

// Domain to ASCII as ada::idna::to_ascii does it, quirks included, over the tables of
// whatwg_tables.go.

// Kinds of mapping runs.
const (
	mapValid = iota
	mapDisallowed
	mapIgnored
	mapDelta // each code point maps to itself plus arg
	mapPairs // alternating upper and lower case: the even ones map to the next code point
	mapSeq   // each code point maps to seq[arg+1 : arg+1+seq[arg]]
)

// Fields of a property value.
const (
	propCCC  = 0xff
	propMark = 1 << 11
)

// Bidi classes as the label check tells them apart, at bits 8 to 10 of a property value.
const (
	bidiOther = iota // WS, B, S, and the explicit embeddings, overrides and isolates
	bidiL
	bidiR // R and AL
	bidiAN
	bidiEN
	bidiNSM
	bidiNeutral // ES, CS, ET, ON and BN
)

// Joining classes, at bits 12 to 14 of a property value.
const (
	joinNone = iota
	joinVirama
	joinR
	joinL
	joinD
)

var idna struct {
	once      sync.Once
	mapStart  []rune
	mapKind   []uint8
	mapArg    []int32
	seq       []rune
	propStart []rune
	prop      []uint16
	compose   map[[2]rune]rune
	parts     map[rune][2]rune
}

func loadIDNA() {
	raw, err := base64.StdEncoding.DecodeString(idnaData)
	if err != nil {
		panic(err)
	}
	data, err := io.ReadAll(flate.NewReader(bytes.NewReader(raw)))
	if err != nil {
		panic(err)
	}
	next := func() int {
		v, n := binary.Uvarint(data)
		data = data[n:]
		return int(v)
	}
	signed := func() rune {
		v := next()
		return rune(v>>1 ^ -(v & 1))
	}
	cp := rune(0)
	for k := next(); k > 0; k-- {
		h := next()
		kind, arg := uint8(h&7), int32(0)
		switch kind {
		case mapDelta:
			arg = signed()
		case mapSeq:
			arg = int32(len(idna.seq))
			n := next()
			idna.seq = append(idna.seq, rune(n))
			c := cp
			for ; n > 0; n-- {
				c += signed()
				idna.seq = append(idna.seq, c)
			}
		}
		idna.mapStart = append(idna.mapStart, cp)
		idna.mapKind = append(idna.mapKind, kind)
		idna.mapArg = append(idna.mapArg, arg)
		cp += rune(h >> 3)
	}
	cp = 0
	for k := next(); k > 0; k-- {
		n := next()
		idna.propStart = append(idna.propStart, cp)
		idna.prop = append(idna.prop, uint16(next()))
		cp += rune(n)
	}
	n := next()
	idna.compose = make(map[[2]rune]rune, n)
	idna.parts = make(map[rune][2]rune, n)
	c := rune(0)
	for ; n > 0; n-- {
		c += rune(next())
		a := c + signed()
		b := a + signed()
		idna.compose[[2]rune{a, b}] = c
		idna.parts[c] = [2]rune{a, b}
	}
}

func run(starts []rune, c rune) int {
	i, found := slices.BinarySearch(starts, c)
	if !found {
		i--
	}
	return i
}

// idnaMap appends the UTS #46 mapping of c to out; ok is false where c is disallowed.
func idnaMap(out []rune, c rune) ([]rune, bool) {
	if c < 0 || c > utf8.MaxRune {
		return out, false
	}
	i := run(idna.mapStart, c)
	switch idna.mapKind[i] {
	case mapValid:
		return append(out, c), true
	case mapDisallowed:
		return out, false
	case mapIgnored:
		return out, true
	case mapDelta:
		return append(out, c+idna.mapArg[i]), true
	case mapPairs:
		if (c-idna.mapStart[i])%2 == 0 {
			return append(out, c+1), true
		}
		return append(out, c), true
	}
	seq := idna.seq[idna.mapArg[i]:]
	return append(out, seq[1:1+seq[0]]...), true
}

// prop is the property value of a code point that mapping can leave.
func prop(c rune) uint16 { return idna.prop[run(idna.propStart, c)] }

func ccc(c rune) int   { return int(prop(c) & propCCC) }
func bidi(c rune) int  { return int(prop(c)>>8) & 7 }
func joins(c rune) int { return int(prop(c)>>12) & 7 }

const (
	hangulS = 0xac00
	hangulL = 0x1100
	hangulV = 0x1161
	hangulT = 0x11a7
	countL  = 19
	countV  = 21
	countT  = 28
	countN  = countV * countT
	countS  = countL * countN
)

func decompose(out []rune, c rune) []rune {
	if c >= hangulS && c < hangulS+countS {
		s := c - hangulS
		out = append(out, hangulL+s/countN, hangulV+(s%countN)/countT)
		if s%countT != 0 {
			out = append(out, hangulT+s%countT)
		}
		return out
	}
	if p, ok := idna.parts[c]; ok {
		return append(decompose(out, p[0]), p[1])
	}
	return append(out, c)
}

// nfc normalizes code points that are all valid, as ada::idna::normalize does. Those can only
// decompose to the pairs that compose back, so the composition table serves both ways.
func nfc(s []rune) []rune {
	in := make([]rune, 0, len(s))
	for _, c := range s {
		in = decompose(in, c)
	}
	for i := 1; i < len(in); i++ {
		k := ccc(in[i])
		if k == 0 {
			continue
		}
		c, j := in[i], i
		for j > 0 && ccc(in[j-1]) > k {
			in[j] = in[j-1]
			j--
		}
		in[j] = c
	}
	n := 0
	for i := 0; i < len(in); i, n = i+1, n+1 {
		c := in[i]
		in[n] = c
		switch {
		case c >= hangulL && c < hangulL+countL:
			if i+1 < len(in) && in[i+1] >= hangulV && in[i+1] < hangulV+countV {
				in[n] = hangulS + ((c-hangulL)*countV+in[i+1]-hangulV)*countT
				i++
				if i+1 < len(in) && in[i+1] > hangulT && in[i+1] < hangulT+countT {
					i++
					in[n] += in[i] - hangulT
				}
			}
		case c >= hangulS && c < hangulS+countS:
			if (c-hangulS)%countT != 0 && i+1 < len(in) && in[i+1] > hangulT && in[i+1] < hangulT+countT {
				i++
				in[n] += in[i] - hangulT
			}
		default:
			start := n
			for last := -1; i+1 < len(in); i++ {
				next := in[i+1]
				k := ccc(next)
				if last < k {
					if c, ok := idna.compose[[2]rune{in[start], next}]; ok {
						in[start] = c
						continue
					}
				}
				if k == 0 {
					break
				}
				last = k
				n++
				in[n] = next
			}
		}
	}
	return in[:n]
}

// labelValid is ada::idna::is_label_valid. Ada returns from the whole check on the first joiner,
// tests the context of a ZWNJ loosely, checks bidi rules only in labels that have right-to-left
// characters, and skips the last character of a left-to-right one.
func labelValid(label []rune) bool {
	if len(label) == 0 {
		return true
	}
	if prop(label[0])&propMark != 0 {
		return false
	}
	for i, c := range label {
		switch c {
		case 0x200c:
			if i > 0 && joins(label[i-1]) == joinVirama {
				return true
			}
			if i == 0 || i+1 >= len(label) {
				return false
			}
			lOrD := func(c rune) bool { j := joins(c); return j == joinL || j == joinD }
			rOrD := func(c rune) bool { j := joins(c); return j == joinR || j == joinD }
			return slices.ContainsFunc(label[:i], lOrD) && slices.ContainsFunc(label[i+1:], rOrD)
		case 0x200d:
			return i > 0 && joins(label[i-1]) == joinVirama
		}
	}
	last := len(label) - 1
	for last >= 0 && bidi(label[last]) == bidiNSM {
		last--
	}
	if last < 0 {
		return false
	}
	if !slices.ContainsFunc(label, func(c rune) bool { d := bidi(c); return d == bidiR || d == bidiAN }) {
		return true
	}
	if bidi(label[0]) == bidiL {
		for _, c := range label[:last] {
			switch bidi(c) {
			case bidiL, bidiEN, bidiNeutral, bidiNSM:
			default:
				return false
			}
		}
		return true
	}
	hasAN, hasEN := false, false
	for i, c := range label[:last+1] {
		d := bidi(c)
		switch d {
		case bidiEN:
			hasEN = true
			if hasAN {
				return false
			}
		case bidiAN:
			hasAN = true
			if hasEN {
				return false
			}
		case bidiR, bidiNeutral, bidiNSM:
		default:
			return false
		}
		if i == last && d != bidiR && d != bidiAN && d != bidiEN {
			return false
		}
	}
	return true
}

// punyLabelValid checks what follows "xn--" in a label: it must decode to valid code points in
// NFC that make a valid label.
func punyLabelValid(s string) bool {
	label, ok := punycodeDecode(s)
	if !ok || len(label) == 0 {
		return false
	}
	for _, c := range label {
		if m, ok := idnaMap(nil, c); !ok || len(m) != 1 || m[0] != c {
			return false
		}
	}
	return slices.Equal(nfc(label), label) && labelValid(label)
}

// domainToASCII is ada::idna::to_ascii on percent-decoded bytes.
func domainToASCII(s []byte) (string, bool) {
	idna.once.Do(loadIDNA)
	ascii := true
	for _, b := range s {
		if b >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		lower := strings.ToLower(string(s))
		for _, label := range strings.Split(lower, ".") {
			if strings.HasPrefix(label, "xn--") && !punyLabelValid(label[4:]) {
				return "", false
			}
		}
		return lower, true
	}
	if !utf8.Valid(s) {
		return "", false
	}
	var mapped []rune
	for _, c := range string(s) {
		var ok bool
		if mapped, ok = idnaMap(mapped, c); !ok {
			return "", false
		}
	}
	var out []byte
	for i, label := range splitRunes(nfc(mapped), '.') {
		if i > 0 {
			out = append(out, '.')
		}
		switch {
		case len(label) >= 4 && string(label[:4]) == "xn--":
			for _, c := range label {
				if c >= 0x80 {
					return "", false
				}
				out = append(out, byte(c))
			}
			if !punyLabelValid(string(label[4:])) {
				return "", false
			}
		case !slices.ContainsFunc(label, func(c rune) bool { return c >= 0x80 }):
			for _, c := range label {
				out = append(out, byte(c))
			}
		default:
			if !labelValid(label) {
				return "", false
			}
			var ok bool
			if out, ok = punycodeEncode(append(out, "xn--"...), label); !ok {
				return "", false
			}
		}
	}
	return string(out), true
}

func splitRunes(s []rune, sep rune) [][]rune {
	var out [][]rune
	for {
		i := slices.Index(s, sep)
		if i < 0 {
			return append(out, s)
		}
		out = append(out, s[:i])
		s = s[i+1:]
	}
}
