package js

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// IsSpace reports whether r is whitespace as a JavaScript `\s` or String.prototype.trim sees it.
// Go's regexp `\s` and unicode.IsSpace are not the same set.
func IsSpace(r rune) bool {
	if r < 128 {
		return r == ' ' || (r >= 9 && r <= 13)
	}
	return r == 0xa0 || r == 0x1680 || (r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 ||
		r == 0x202f || r == 0x205f || r == 0x3000 || r == 0xfeff
}

// IsLineEnd reports whether r is a line terminator: what a regex `.` does not match.
func IsLineEnd(r rune) bool { return r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029 }

// IsWord reports whether c is a character of `\w`.
func IsWord(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

// IsDigit reports whether c is a character of `\d`.
func IsDigit(c rune) bool { return c >= '0' && c <= '9' }

// Trim is String.prototype.trim.
func Trim(s string) string { return strings.TrimFunc(s, IsSpace) }

// TrimStart is String.prototype.trimStart.
func TrimStart(s string) string { return strings.TrimLeftFunc(s, IsSpace) }

// TrimEnd is String.prototype.trimEnd.
func TrimEnd(s string) string { return strings.TrimRightFunc(s, IsSpace) }

// IsBlank reports whether s is empty after trimming.
func IsBlank(s string) bool { return Trim(s) == "" }

// Len is s.length: the number of UTF-16 code units.
func Len(s string) int {
	n := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			n++
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
		i += size
	}
	return n
}

// UnitsBefore returns the number of UTF-16 code units in s[:i], for a byte offset i.
func UnitsBefore(s string, i int) int { return Len(s[:i]) }

// ByteOffset returns the byte offset of UTF-16 code unit index u in s. An index inside a
// surrogate pair rounds up to the end of its character; one past the end gives len(s).
func ByteOffset(s string, u int) int {
	if u <= 0 {
		return 0
	}
	n := 0
	for i := 0; i < len(s); {
		if n >= u {
			return i
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
		i += size
	}
	return len(s)
}

// Slice is s.slice(start, end) in UTF-16 code units, with the negative-index rules of slice.
// A cut through a surrogate pair keeps the whole character on the side it starts.
func Slice(s string, start, end int) string {
	n := Len(s)
	if start < 0 {
		start += n
		if start < 0 {
			start = 0
		}
	}
	if end < 0 {
		end += n
		if end < 0 {
			end = 0
		}
	}
	if start > n {
		start = n
	}
	if end > n {
		end = n
	}
	if start >= end {
		return ""
	}
	return s[ByteOffset(s, start):ByteOffset(s, end)]
}

// SliceFrom is s.slice(start).
func SliceFrom(s string, start int) string { return Slice(s, start, Len(s)) }

// Units returns s as UTF-16 code units, for code that indexes text the way charCodeAt does.
func Units(s string) []uint16 { return utf16.Encode([]rune(s)) }

// FromUnits turns UTF-16 code units back into a string. A lone surrogate becomes U+FFFD.
func FromUnits(u []uint16) string { return string(utf16.Decode(u)) }

// CodeUnitAt is s.charCodeAt(i) for a UTF-16 index, or -1 past the end.
func CodeUnitAt(s string, i int) int {
	if i < 0 {
		return -1
	}
	n := 0
	for j := 0; j < len(s); {
		r, size := utf8.DecodeRuneInString(s[j:])
		if r >= 0x10000 {
			hi, lo := utf16.EncodeRune(r)
			if n == i {
				return int(hi)
			}
			if n+1 == i {
				return int(lo)
			}
			n += 2
		} else {
			if n == i {
				return int(r)
			}
			n++
		}
		j += size
	}
	return -1
}

// ToLower is String.prototype.toLowerCase, where it differs from strings.ToLower for one letter
// that lowercases to two code points.
func ToLower(s string) string {
	if strings.ContainsRune(s, 'İ') {
		s = strings.ReplaceAll(s, "İ", "i̇")
	}
	return strings.ToLower(s)
}

// ToUpper is String.prototype.toUpperCase for the common cases.
func ToUpper(s string) string {
	if strings.ContainsRune(s, 'ß') {
		s = strings.ReplaceAll(s, "ß", "SS")
	}
	return strings.ToUpper(s)
}

// ASCIILower lowercases A-Z only, as a case-insensitive regex without the `u` flag compares
// an ASCII pattern.
func ASCIILower(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// HasPrefixFold reports whether s starts with prefix, ignoring ASCII case. prefix must be lowercase.
func HasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

// RuneAt decodes the character at byte offset i, or returns -1 at or past the end.
func RuneAt(s string, i int) (rune, int) {
	if i < 0 || i >= len(s) {
		return -1, 0
	}
	if c := s[i]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeRuneInString(s[i:])
}

// RuneBefore decodes the character that ends at byte offset i, or returns -1 at the start.
func RuneBefore(s string, i int) (rune, int) {
	if i <= 0 || i > len(s) {
		return -1, 0
	}
	if c := s[i-1]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeLastRuneInString(s[:i])
}

// Repeat is String.prototype.repeat, which gives "" for a count below one.
func Repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
}

// Split is String.prototype.split with a string separator.
func Split(s, sep string) []string {
	if sep == "" {
		out := []string{}
		for _, r := range s {
			if r >= 0x10000 {
				// Each half of a pair is its own piece in JavaScript; Go cannot hold a lone half.
				out = append(out, string(r))
				continue
			}
			out = append(out, string(r))
		}
		return out
	}
	return strings.Split(s, sep)
}
