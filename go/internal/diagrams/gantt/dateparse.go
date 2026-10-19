package gantt

import (
	"math"

	"github.com/alessioalex/pele/go/internal/js"
)

// parseDateString is `new Date(str)` for a string: V8's date parser, which accepts the ES5 ISO
// format and falls back to its legacy rules for anything else.

const (
	dpNone               = math.MaxInt32
	dpMaxSignificant     = 9
	tagInvalid           = -6
	tagUnknown           = -5
	tagWhiteSpace        = -4
	tagNumber            = -3
	tagSymbol            = -2
	tagEndOfInput        = -1
	kwInvalid            = 0
	kwMonthName          = 1
	kwTimeZoneName       = 2
	kwTimeSeparator      = 3
	kwAmPm               = 4
	smiMax           int = math.MaxInt32
)

type dpToken struct{ tag, length, value int }

func (t dpToken) isInvalid() bool                { return t.tag == tagInvalid }
func (t dpToken) isNumber() bool                 { return t.tag == tagNumber }
func (t dpToken) isSymbol(c int) bool            { return t.tag == tagSymbol && t.value == c }
func (t dpToken) isWhiteSpace() bool             { return t.tag == tagWhiteSpace }
func (t dpToken) isEnd() bool                    { return t.tag == tagEndOfInput }
func (t dpToken) isKeyword() bool                { return t.tag >= 0 }
func (t dpToken) isFixedLengthNumber(n int) bool { return t.tag == tagNumber && t.length == n }
func (t dpToken) isAsciiSign() bool              { return t.tag == tagSymbol && (t.value == '-' || t.value == '+') }
func (t dpToken) asciiSign() int                 { return 44 - t.value }
func (t dpToken) isKeywordZ() bool               { return t.tag == kwTimeZoneName && t.length == 1 && t.value == 0 }

// V8's WhiteSpace, without the line terminators.
func dpWhiteSpace(c rune) bool {
	return js.IsSpace(c) && c != '\n' && c != '\r' && c != 0x2028 && c != 0x2029
}

type dpReader struct {
	s     []uint16
	index int
	ch    rune
}

func (r *dpReader) next() {
	if r.index < len(r.s) {
		r.ch = rune(r.s[r.index])
	} else {
		r.ch = 0
	}
	r.index++
}

func (r *dpReader) digit() bool { return r.ch >= '0' && r.ch <= '9' }

func (r *dpReader) skip(c rune) bool {
	if r.ch == c {
		r.next()
		return true
	}
	return false
}

func (r *dpReader) readUnsignedNumeral() int {
	n, i := 0, 0
	for r.ch == '0' {
		r.next()
	}
	for r.digit() {
		if i < dpMaxSignificant {
			n = n*10 + int(r.ch-'0')
		}
		i++
		r.next()
	}
	return n
}

func (r *dpReader) wordChar() bool { return r.ch >= 'A' && !dpWhiteSpace(r.ch) }

var dpKeywords = []struct {
	prefix [3]rune
	typ    int
	value  int
}{
	{[3]rune{'j', 'a', 'n'}, kwMonthName, 1},
	{[3]rune{'f', 'e', 'b'}, kwMonthName, 2},
	{[3]rune{'m', 'a', 'r'}, kwMonthName, 3},
	{[3]rune{'a', 'p', 'r'}, kwMonthName, 4},
	{[3]rune{'m', 'a', 'y'}, kwMonthName, 5},
	{[3]rune{'j', 'u', 'n'}, kwMonthName, 6},
	{[3]rune{'j', 'u', 'l'}, kwMonthName, 7},
	{[3]rune{'a', 'u', 'g'}, kwMonthName, 8},
	{[3]rune{'s', 'e', 'p'}, kwMonthName, 9},
	{[3]rune{'o', 'c', 't'}, kwMonthName, 10},
	{[3]rune{'n', 'o', 'v'}, kwMonthName, 11},
	{[3]rune{'d', 'e', 'c'}, kwMonthName, 12},
	{[3]rune{'a', 'm', 0}, kwAmPm, 0},
	{[3]rune{'p', 'm', 0}, kwAmPm, 12},
	{[3]rune{'u', 't', 0}, kwTimeZoneName, 0},
	{[3]rune{'u', 't', 'c'}, kwTimeZoneName, 0},
	{[3]rune{'z', 0, 0}, kwTimeZoneName, 0},
	{[3]rune{'g', 'm', 't'}, kwTimeZoneName, 0},
	{[3]rune{'c', 'd', 't'}, kwTimeZoneName, -5},
	{[3]rune{'c', 's', 't'}, kwTimeZoneName, -6},
	{[3]rune{'e', 'd', 't'}, kwTimeZoneName, -4},
	{[3]rune{'e', 's', 't'}, kwTimeZoneName, -5},
	{[3]rune{'m', 'd', 't'}, kwTimeZoneName, -6},
	{[3]rune{'m', 's', 't'}, kwTimeZoneName, -7},
	{[3]rune{'p', 'd', 't'}, kwTimeZoneName, -7},
	{[3]rune{'p', 's', 't'}, kwTimeZoneName, -8},
	{[3]rune{'t', 0, 0}, kwTimeSeparator, 0},
}

func dpLookup(prefix [3]rune, length int) (int, int) {
	for _, k := range dpKeywords {
		if k.prefix == prefix && (length <= 3 || k.typ == kwMonthName) {
			return k.typ, k.value
		}
	}
	return kwInvalid, 0
}

type dpScanner struct {
	in   *dpReader
	peek dpToken
}

func (s *dpScanner) scan() dpToken {
	in := s.in
	pre := in.index
	if in.ch == 0 {
		return dpToken{tagEndOfInput, 0, -1}
	}
	if in.digit() {
		n := in.readUnsignedNumeral()
		return dpToken{tagNumber, in.index - pre, n}
	}
	for _, c := range [...]rune{':', '-', '+', '.', ')'} {
		if in.skip(c) {
			return dpToken{tagSymbol, 1, int(c)}
		}
	}
	if in.wordChar() {
		var prefix [3]rune
		length := 0
		for ; in.wordChar(); in.next() {
			if length < 3 {
				prefix[length] = in.ch | 0x20
			}
			length++
		}
		typ, value := dpLookup(prefix, length)
		return dpToken{typ, length, value}
	}
	if js.IsSpace(in.ch) {
		in.next()
		return dpToken{tagWhiteSpace, in.index - pre, 0}
	}
	if in.ch == '(' {
		balance := 0
		for {
			if in.ch == ')' {
				balance--
			} else if in.ch == '(' {
				balance++
			}
			in.next()
			if !(balance > 0 && in.ch != 0) {
				break
			}
		}
		return dpToken{tagUnknown, 1, -1}
	}
	in.next()
	return dpToken{tagUnknown, 1, -1}
}

func (s *dpScanner) next() dpToken {
	t := s.peek
	s.peek = s.scan()
	return t
}

func (s *dpScanner) skipSymbol(c int) bool {
	if s.peek.isSymbol(c) {
		s.peek = s.scan()
		return true
	}
	return false
}

func between(x, lo, hi int) bool { return x >= lo && x <= hi }

type dpTimeZone struct{ sign, hour, minute int }

func (z *dpTimeZone) set(offsetHours int) {
	if offsetHours < 0 {
		z.sign = -1
	} else {
		z.sign = 1
	}
	z.hour = offsetHours * z.sign
	z.minute = 0
}

func (z *dpTimeZone) setSign(sign int) {
	if sign < 0 {
		z.sign = -1
	} else {
		z.sign = 1
	}
}

func (z *dpTimeZone) isExpecting(n int) bool {
	return z.hour != dpNone && z.minute == dpNone && between(n, 0, 59)
}

func (z *dpTimeZone) isUTC() bool   { return z.hour == 0 && z.minute == 0 }
func (z *dpTimeZone) isEmpty() bool { return z.hour == dpNone }

func (z *dpTimeZone) write() (float64, bool) {
	if z.sign == dpNone {
		return math.NaN(), true
	}
	if z.hour == dpNone {
		z.hour = 0
	}
	if z.minute == dpNone {
		z.minute = 0
	}
	total := uint32(z.hour)*3600 + uint32(z.minute)*60
	if uint64(total) > uint64(smiMax) {
		return 0, false
	}
	seconds := int(total)
	if z.sign < 0 {
		seconds = -seconds
	}
	return float64(seconds), true
}

type dpTime struct {
	comp       [4]int
	index      int
	hourOffset int
}

func (t *dpTime) isEmpty() bool { return t.index == 0 }

func (t *dpTime) isExpecting(n int) bool {
	return (t.index == 1 && between(n, 0, 59)) || (t.index == 2 && between(n, 0, 59)) ||
		(t.index == 3 && between(n, 0, 999))
}

func (t *dpTime) add(n int) bool {
	if t.index < 4 {
		t.comp[t.index] = n
		t.index++
		return true
	}
	return false
}

func (t *dpTime) addFinal(n int) bool {
	if !t.add(n) {
		return false
	}
	for t.index < 4 {
		t.comp[t.index] = 0
		t.index++
	}
	return true
}

func (t *dpTime) write() ([4]int, bool) {
	for t.index < 4 {
		t.comp[t.index] = 0
		t.index++
	}
	hour, minute, second, ms := t.comp[0], t.comp[1], t.comp[2], t.comp[3]
	if t.hourOffset != dpNone {
		if !between(hour, 0, 12) {
			return t.comp, false
		}
		hour %= 12
		hour += t.hourOffset
	}
	if !between(hour, 0, 23) || !between(minute, 0, 59) || !between(second, 0, 59) || !between(ms, 0, 999) {
		if hour != 24 || minute != 0 || second != 0 || ms != 0 {
			return t.comp, false
		}
	}
	return [4]int{hour, minute, second, ms}, true
}

type dpDay struct {
	comp       [3]int
	index      int
	namedMonth int
	isoDate    bool
}

func (d *dpDay) isEmpty() bool { return d.index == 0 }

func (d *dpDay) add(n int) bool {
	if d.index < 3 {
		d.comp[d.index] = n
		d.index++
		return true
	}
	return false
}

func (d *dpDay) write() (year, month, day int, ok bool) {
	if d.index < 1 {
		return 0, 0, 0, false
	}
	for d.index < 3 {
		d.comp[d.index] = 1
		d.index++
	}
	year = 0
	month, day = dpNone, dpNone
	if d.namedMonth == dpNone {
		if d.isoDate || (d.index == 3 && !between(d.comp[0], 1, 31)) {
			year, month, day = d.comp[0], d.comp[1], d.comp[2]
		} else {
			month, day = d.comp[0], d.comp[1]
			if d.index == 3 {
				year = d.comp[2]
			}
		}
	} else {
		month = d.namedMonth
		if d.index == 1 {
			day = d.comp[0]
		} else if !between(d.comp[0], 1, 31) {
			year, day = d.comp[0], d.comp[1]
		} else {
			day, year = d.comp[0], d.comp[1]
		}
	}
	if !d.isoDate {
		if between(year, 0, 49) {
			year += 2000
		} else if between(year, 50, 99) {
			year += 1900
		}
	}
	if !between(month, 1, 12) || !between(day, 1, 31) {
		return 0, 0, 0, false
	}
	return year, month - 1, day, true
}

func readMilliseconds(t dpToken) int {
	number, length := t.value, t.length
	if length < 3 {
		if length == 1 {
			number *= 100
		} else if length == 2 {
			number *= 10
		}
	} else if length > 3 {
		if length > dpMaxSignificant {
			length = dpMaxSignificant
		}
		factor := 1
		for {
			factor *= 10
			length--
			if length <= 3 {
				break
			}
		}
		number /= factor
	}
	return number
}

func parseES5(s *dpScanner, day *dpDay, tm *dpTime, tz *dpTimeZone) dpToken {
	if s.peek.isAsciiSign() {
		signToken := s.next()
		if !s.peek.isFixedLengthNumber(6) {
			return signToken
		}
		sign := signToken.asciiSign()
		year := s.next().value
		if sign < 0 && year == 0 {
			return signToken
		}
		day.add(sign * year)
	} else if s.peek.isFixedLengthNumber(4) {
		day.add(s.next().value)
	} else {
		return s.next()
	}
	if s.skipSymbol('-') {
		if !s.peek.isFixedLengthNumber(2) || !between(s.peek.value, 1, 12) {
			return s.next()
		}
		day.add(s.next().value)
		if s.skipSymbol('-') {
			if !s.peek.isFixedLengthNumber(2) || !between(s.peek.value, 1, 31) {
				return s.next()
			}
			day.add(s.next().value)
		}
	}
	invalid := dpToken{tagInvalid, 0, -1}
	if s.peek.tag != kwTimeSeparator {
		if !s.peek.isEnd() {
			return s.next()
		}
	} else {
		s.next()
		if !s.peek.isFixedLengthNumber(2) || !between(s.peek.value, 0, 24) {
			return invalid
		}
		hourIs24 := s.peek.value == 24
		tm.add(s.next().value)
		if !s.skipSymbol(':') {
			return invalid
		}
		if !s.peek.isFixedLengthNumber(2) || !between(s.peek.value, 0, 59) || (hourIs24 && s.peek.value > 0) {
			return invalid
		}
		tm.add(s.next().value)
		if s.skipSymbol(':') {
			if !s.peek.isFixedLengthNumber(2) || !between(s.peek.value, 0, 59) || (hourIs24 && s.peek.value > 0) {
				return invalid
			}
			tm.add(s.next().value)
			if s.skipSymbol('.') {
				if !s.peek.isNumber() || (hourIs24 && s.peek.value > 0) {
					return invalid
				}
				tm.add(readMilliseconds(s.next()))
			}
		}
		if s.peek.isKeywordZ() {
			s.next()
			tz.set(0)
		} else if s.peek.isSymbol('+') || s.peek.isSymbol('-') {
			if s.next().value == '+' {
				tz.setSign(1)
			} else {
				tz.setSign(-1)
			}
			if s.peek.isFixedLengthNumber(4) {
				hourmin := s.next().value
				hour, min := hourmin/100, hourmin%100
				if !between(hour, 0, 23) || !between(min, 0, 59) {
					return invalid
				}
				tz.hour, tz.minute = hour, min
			} else {
				if !s.peek.isFixedLengthNumber(2) || !between(s.peek.value, 0, 23) {
					return invalid
				}
				tz.hour = s.next().value
				if !s.skipSymbol(':') {
					return invalid
				}
				if !s.peek.isFixedLengthNumber(2) || !between(s.peek.value, 0, 59) {
					return invalid
				}
				tz.minute = s.next().value
			}
		}
		if !s.peek.isEnd() {
			return invalid
		}
	}
	if tz.isEmpty() && tm.isEmpty() {
		tz.set(0)
	}
	day.isoDate = true
	return dpToken{tagEndOfInput, 0, -1}
}

// parseDateFields runs V8's DateParser::Parse, returning year, month, day, hours, minutes,
// seconds, milliseconds and the UTC offset in seconds (NaN when none was given).
func parseDateFields(str string) ([8]float64, bool) {
	var out [8]float64
	in := &dpReader{s: js.Units(str)}
	in.next()
	s := &dpScanner{in: in}
	s.peek = s.scan()
	tz := &dpTimeZone{dpNone, dpNone, dpNone}
	tm := &dpTime{hourOffset: dpNone}
	day := &dpDay{namedMonth: dpNone}

	token := parseES5(s, day, tm, tz)
	if token.isInvalid() {
		return out, false
	}
	hasReadNumber := !day.isEmpty()
	for ; !token.isEnd(); token = s.next() {
		switch {
		case token.isNumber():
			hasReadNumber = true
			n := token.value
			if s.skipSymbol(':') {
				if s.skipSymbol(':') {
					if !tm.isEmpty() {
						return out, false
					}
					tm.add(n)
					tm.add(0)
				} else {
					if !tm.add(n) {
						return out, false
					}
					if s.peek.isSymbol('.') {
						s.next()
					}
				}
			} else if s.skipSymbol('.') && tm.isExpecting(n) {
				tm.add(n)
				if !s.peek.isNumber() {
					return out, false
				}
				ms := readMilliseconds(s.next())
				if ms < 0 {
					return out, false
				}
				tm.addFinal(ms)
			} else if tz.isExpecting(n) {
				tz.minute = n
			} else if tm.isExpecting(n) {
				tm.addFinal(n)
				peek := s.peek
				if !peek.isEnd() && !peek.isWhiteSpace() && !peek.isKeywordZ() && !peek.isAsciiSign() {
					return out, false
				}
			} else {
				if !day.add(n) {
					return out, false
				}
				s.skipSymbol('-')
			}
		case token.isKeyword():
			if token.tag == kwAmPm && !tm.isEmpty() {
				tm.hourOffset = token.value
			} else if token.tag == kwMonthName {
				day.namedMonth = token.value
				s.skipSymbol('-')
			} else if token.tag == kwTimeZoneName && hasReadNumber {
				tz.set(token.value)
			} else {
				if hasReadNumber {
					return out, false
				}
				if s.peek.isNumber() {
					return out, false
				}
			}
		case token.isAsciiSign() && (tz.isUTC() || !tm.isEmpty()):
			tz.setSign(token.asciiSign())
			n, length := 0, 0
			if s.peek.isNumber() {
				t := s.next()
				length, n = t.length, t.value
			}
			hasReadNumber = true
			if s.peek.isSymbol(':') {
				tz.hour = n
				tz.minute = dpNone
			} else if length == 2 || length == 1 {
				tz.hour, tz.minute = n, 0
			} else if length == 4 || length == 3 {
				tz.hour, tz.minute = n/100, n%100
			} else {
				return out, false
			}
		case (token.isAsciiSign() || token.isSymbol(')')) && hasReadNumber:
			return out, false
		}
	}

	year, month, dd, ok := day.write()
	if !ok {
		return out, false
	}
	t, ok := tm.write()
	if !ok {
		return out, false
	}
	offset, ok := tz.write()
	if !ok {
		return out, false
	}
	out = [8]float64{float64(year), float64(month), float64(dd), float64(t[0]), float64(t[1]), float64(t[2]), float64(t[3]), offset}
	return out, true
}

// parseDateString is `new Date(str)`.
func parseDateString(str string) *jsDate {
	f, ok := parseDateFields(str)
	if !ok {
		return &jsDate{math.NaN()}
	}
	date := makeDate(makeDay(f[0], f[1], f[2]), makeTime(f[3], f[4], f[5], f[6]))
	if math.IsNaN(f[7]) {
		if date < -maxTimeBeforeUTCInMs || date > maxTimeBeforeUTCInMs {
			return &jsDate{math.NaN()}
		}
		date = toUTC(date)
	} else {
		date -= f[7] * 1000
		if date < -maxTimeInMs || date > maxTimeInMs {
			return &jsDate{math.NaN()}
		}
	}
	return &jsDate{timeClip(date)}
}
