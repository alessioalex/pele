package util

import (
	"slices"
	"strings"
)

// Punycode (RFC 3492) as ada::idna has it, overflow checks included.
const (
	punyBase = 36
	punyTMin = 1
	punyTMax = 26
	punySkew = 38
	punyDamp = 700
)

func punyAdapt(d, n int32, first bool) int32 {
	if first {
		d /= punyDamp
	} else {
		d /= 2
	}
	d += d / n
	k := int32(0)
	for d > ((punyBase-punyTMin)*punyTMax)/2 {
		d /= punyBase - punyTMin
		k += punyBase
	}
	return k + ((punyBase-punyTMin+1)*d)/(d+punySkew)
}

func punyThreshold(k, bias int32) int32 {
	switch {
	case k <= bias:
		return punyTMin
	case k >= bias+punyTMax:
		return punyTMax
	}
	return k - bias
}

// punycodeDecode decodes a label without its "xn--". Like ada, it takes lowercase digits only and
// leaves the code points it gives unchecked: the IDNA mapping refuses any out of range.
func punycodeDecode(s string) ([]rune, bool) {
	var out []rune
	n := uint32(0x80)
	i := int32(0)
	bias := int32(72)
	if end := strings.LastIndexByte(s, '-'); end >= 0 {
		for j := 0; j < end; j++ {
			if s[j] >= 0x80 {
				return nil, false
			}
			out = append(out, rune(s[j]))
		}
		s = s[end+1:]
	}
	for len(s) > 0 {
		oldi := i
		w := int32(1)
		for k := int32(punyBase); ; k += punyBase {
			if len(s) == 0 {
				return nil, false
			}
			c := s[0]
			s = s[1:]
			var digit int32
			switch {
			case c >= 'a' && c <= 'z':
				digit = int32(c - 'a')
			case c >= '0' && c <= '9':
				digit = int32(c-'0') + 26
			default:
				return nil, false
			}
			if digit > (0x7fffffff-i)/w {
				return nil, false
			}
			i += digit * w
			t := punyThreshold(k, bias)
			if digit < t {
				break
			}
			if w > 0x7fffffff/(punyBase-t) {
				return nil, false
			}
			w *= punyBase - t
		}
		count := int32(len(out)) + 1
		bias = punyAdapt(i-oldi, count, oldi == 0)
		if i/count > int32(0x7fffffff-n) {
			return nil, false
		}
		n += uint32(i / count)
		i %= count
		if n < 0x80 {
			return nil, false
		}
		out = slices.Insert(out, int(i), rune(n))
		i++
	}
	return out, true
}

// punycodeEncode appends the encoding of a label to dst.
func punycodeEncode(dst []byte, label []rune) ([]byte, bool) {
	n := uint32(0x80)
	d := int32(0)
	bias := int32(72)
	h := 0
	for _, c := range label {
		if c < 0x80 {
			h++
			dst = append(dst, byte(c))
		}
		if c > 0x10ffff || (c >= 0xd880 && c < 0xe000) {
			return dst, false
		}
	}
	b := h
	if b > 0 {
		dst = append(dst, '-')
	}
	digit := func(v int32) byte {
		if v < 26 {
			return byte(v + 'a')
		}
		return byte(v + 22)
	}
	for h < len(label) {
		m := uint32(0x10ffff)
		for _, c := range label {
			if uint32(c) >= n && uint32(c) < m {
				m = uint32(c)
			}
		}
		if uint64(m-n) > uint64(0x7fffffff-d)/uint64(h+1) {
			return dst, false
		}
		d += int32((m - n) * uint32(h+1))
		n = m
		for _, c := range label {
			if uint32(c) < n {
				if d == 0x7fffffff {
					return dst, false
				}
				d++
			}
			if uint32(c) == n {
				q := d
				for k := int32(punyBase); ; k += punyBase {
					t := punyThreshold(k, bias)
					if q < t {
						break
					}
					dst = append(dst, digit(t+(q-t)%(punyBase-t)))
					q = (q - t) / (punyBase - t)
				}
				dst = append(dst, digit(q))
				bias = punyAdapt(d, int32(h+1), h == b)
				d = 0
				h++
			}
		}
		d++
		n++
	}
	return dst, true
}
