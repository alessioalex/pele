// Package js reproduces the JavaScript semantics that Pele's output depends on, so that the Go
// port draws exactly what the TypeScript library draws.
package js

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// Round is Math.round: halves go toward +Infinity.
func Round(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	if r == 0 && math.Signbit(x) {
		return math.Copysign(0, -1)
	}
	return r
}

// Sign is Math.sign.
func Sign(x float64) float64 {
	switch {
	case math.IsNaN(x):
		return x
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return x
}

// Trunc is Math.trunc.
func Trunc(x float64) float64 { return math.Trunc(x) }

// Hypot is Math.hypot as V8 computes it: scaled to the largest value, with Kahan summation.
func Hypot(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxv := 0.0
	nan := false
	abs := make([]float64, len(values))
	for i, v := range values {
		if math.IsNaN(v) {
			nan = true
			continue
		}
		a := math.Abs(v)
		abs[i] = a
		if a > maxv {
			maxv = a
		}
	}
	if math.IsInf(maxv, 1) {
		return math.Inf(1)
	}
	if nan {
		return math.NaN()
	}
	if maxv == 0 {
		return 0
	}
	sum, comp := 0.0, 0.0
	for _, a := range abs {
		n := a / maxv
		summand := float64(n*n) - comp
		pre := sum + summand
		comp = (pre - sum) - summand
		sum = pre
	}
	return math.Sqrt(sum) * maxv
}

// ToInt32 is the conversion behind `x | 0`.
func ToInt32(x float64) int32 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0
	}
	t := math.Trunc(x)
	m := math.Mod(t, 4294967296)
	if m < 0 {
		m += 4294967296
	}
	return int32(uint32(m))
}

// ToUint32 is the conversion behind `x >>> 0`.
func ToUint32(x float64) uint32 { return uint32(ToInt32(x)) }

// IsInteger is Number.isInteger.
func IsInteger(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0) && math.Trunc(x) == x
}

// IsFinite is Number.isFinite.
func IsFinite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// NumberToString is String(x) for a number.
func NumberToString(x float64) string {
	switch {
	case math.IsNaN(x):
		return "NaN"
	case x == 0:
		return "0"
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	}
	if x == math.Trunc(x) && math.Abs(x) < 1e15 {
		return strconv.FormatInt(int64(x), 10)
	}
	sign := ""
	if x < 0 {
		sign = "-"
		x = -x
	}
	e := strconv.FormatFloat(x, 'e', -1, 64)
	at := strings.IndexByte(e, 'e')
	mant, exp := e[:at], e[at+1:]
	digits := strings.Replace(mant, ".", "", 1)
	k := len(digits)
	ex, _ := strconv.Atoi(exp)
	n := ex + 1
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}
	es := "+"
	if n-1 < 0 {
		es = "-"
	}
	ab := n - 1
	if ab < 0 {
		ab = -ab
	}
	if k == 1 {
		return sign + digits + "e" + es + strconv.Itoa(ab)
	}
	return sign + digits[:1] + "." + digits[1:] + "e" + es + strconv.Itoa(ab)
}

// Num is a shorthand for NumberToString, for the many template literals that print a number.
func Num(x float64) string { return NumberToString(x) }

// exactDigits returns the decimal digits of a positive finite x, without leading or trailing
// zeros, and the position of the decimal point: x = 0.digits × 10^point.
func exactDigits(x float64) (string, int) {
	e := strconv.FormatFloat(x, 'e', 1100, 64)
	at := strings.IndexByte(e, 'e')
	mant, exp := e[:at], e[at+1:]
	digits := strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
	ex, _ := strconv.Atoi(exp)
	if digits == "" {
		return "0", 1
	}
	return digits, ex + 1
}

// roundDigits rounds digits (0.digits × 10^point) to keep `keep` digits after position 0,
// halves away from zero. It returns the rounded digits, padded with zeros to `keep`, and the
// new point.
func roundDigits(digits string, point, keep int) (string, int) {
	if keep < 0 {
		return "", point
	}
	if len(digits) <= keep {
		return digits + strings.Repeat("0", keep-len(digits)), point
	}
	up := digits[keep] >= '5'
	b := []byte(digits[:keep])
	if up {
		i := len(b) - 1
		for ; i >= 0; i-- {
			if b[i] == '9' {
				b[i] = '0'
			} else {
				b[i]++
				break
			}
		}
		if i < 0 {
			b = append([]byte{'1'}, b...)
			point++
			b = b[:keep]
			if keep == 0 {
				b = []byte{}
			}
		}
	}
	return string(b), point
}

// ToFixed is Number.prototype.toFixed: of the two nearest, the larger.
func ToFixed(x float64, f int) string {
	if math.IsNaN(x) {
		return "NaN"
	}
	if math.Abs(x) >= 1e21 || math.IsInf(x, 0) {
		return NumberToString(x)
	}
	sign := ""
	if x < 0 {
		sign = "-"
		x = -x
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(f)), nil)
	v := new(big.Float).SetPrec(uint(64 + 4*f + 64)).SetFloat64(x)
	v.Mul(v, new(big.Float).SetInt(scale))
	v.Add(v, big.NewFloat(0.5))
	n, _ := v.Int(nil)
	digits := n.String()
	if len(digits) <= f {
		digits = strings.Repeat("0", f+1-len(digits)) + digits
	}
	if f == 0 {
		return sign + digits
	}
	return sign + digits[:len(digits)-f] + "." + digits[len(digits)-f:]
}

// ToPrecision is Number.prototype.toPrecision.
func ToPrecision(x float64, p int) string {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return NumberToString(x)
	}
	sign := ""
	if x < 0 {
		sign = "-"
		x = -x
	}
	var m string
	e := 0
	if x == 0 {
		m = strings.Repeat("0", p)
	} else {
		digits, point := exactDigits(x)
		m, point = roundDigits(digits, point, p)
		e = point - 1
	}
	if e < -6 || e >= p {
		out := m[:1]
		if p != 1 {
			out += "." + m[1:]
		}
		if e > 0 {
			return sign + out + "e+" + strconv.Itoa(e)
		}
		return sign + out + "e-" + strconv.Itoa(-e)
	}
	if e == p-1 {
		return sign + m
	}
	if e >= 0 {
		return sign + m[:e+1] + "." + m[e+1:]
	}
	return sign + "0." + strings.Repeat("0", -(e+1)) + m
}

// decimalPrefix returns the length of the longest StrDecimalLiteral at the start of s, without
// Infinity, or 0.
func decimalPrefix(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	intDigits := i - start
	fracDigits := 0
	if i < len(s) && s[i] == '.' {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		fracDigits = j - i - 1
		if intDigits > 0 || fracDigits > 0 {
			i = j
		}
	}
	if intDigits == 0 && fracDigits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		k := j
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		if k > j {
			i = k
		}
	}
	return i
}

func parseDecimal(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return v
		}
		return math.NaN()
	}
	return v
}

// ParseFloat is the global parseFloat.
func ParseFloat(s string) float64 {
	s = TrimStart(s)
	rest := s
	neg := false
	if strings.HasPrefix(rest, "+") || strings.HasPrefix(rest, "-") {
		neg = rest[0] == '-'
		rest = rest[1:]
	}
	if strings.HasPrefix(rest, "Infinity") {
		if neg {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	n := decimalPrefix(s)
	if n == 0 {
		return math.NaN()
	}
	return parseDecimal(s[:n])
}

func radixValue(digits string, radix int) float64 {
	v, ok := new(big.Int).SetString(digits, radix)
	if !ok {
		return math.NaN()
	}
	f, _ := new(big.Float).SetInt(v).Float64()
	return f
}

// ToNumber is Number(s) for a string.
func ToNumber(s string) float64 {
	s = Trim(s)
	if s == "" {
		return 0
	}
	if len(s) > 2 && s[0] == '0' {
		radix := 0
		switch s[1] {
		case 'x', 'X':
			radix = 16
		case 'o', 'O':
			radix = 8
		case 'b', 'B':
			radix = 2
		}
		if radix != 0 {
			for i := 2; i < len(s); i++ {
				if digitValue(s[i]) >= radix {
					return math.NaN()
				}
			}
			return radixValue(s[2:], radix)
		}
	}
	switch s {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if decimalPrefix(s) != len(s) {
		return math.NaN()
	}
	return parseDecimal(s)
}

func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return 99
}

// ParseInt is the global parseInt. A radix of 0 means none was given.
func ParseInt(s string, radix int) float64 {
	s = TrimStart(s)
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	strip := true
	if radix != 0 {
		if radix < 2 || radix > 36 {
			return math.NaN()
		}
		if radix != 16 {
			strip = false
		}
	} else {
		radix = 10
	}
	if strip && len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
		radix = 16
	}
	end := 0
	for end < len(s) && digitValue(s[end]) < radix {
		end++
	}
	if end == 0 {
		return math.NaN()
	}
	var v float64
	if radix == 10 {
		v = parseDecimal(s[:end])
	} else {
		v = radixValue(s[:end], radix)
	}
	if neg {
		return -v
	}
	return v
}

// Cmp turns the result of a JavaScript sort comparator into an int for slices.SortStableFunc.
// A NaN result counts as equal, as it does in Array.prototype.sort.
func Cmp(d float64) int {
	switch {
	case d < 0:
		return -1
	case d > 0:
		return 1
	}
	return 0
}
