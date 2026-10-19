package util

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// whatwgHref parses an http: or https: URL as the WHATWG URL parser does and returns its
// serialization, as `new URL(s).toString()` does, with the scheme and host lowercased. ok is
// false where the URL constructor throws. It follows ada, the parser inside Node; other schemes
// never reach it and are refused.
func whatwgHref(s string) (string, bool) {
	in := []rune(s)
	start, end := 0, len(in)
	for start < end && in[start] <= ' ' {
		start++
	}
	for end > start && in[end-1] <= ' ' {
		end--
	}
	in = slices.DeleteFunc(slices.Clone(in[start:end]), func(c rune) bool { return c == '\t' || c == '\n' || c == '\r' })

	colon := slices.Index(in, ':')
	if colon < 0 {
		return "", false
	}
	scheme := strings.ToLower(string(in[:colon]))
	var port int
	switch scheme {
	case "http":
		port = 80
	case "https":
		port = 443
	default:
		return "", false
	}
	rest := in[colon+1:]
	var fragment []rune
	hasFragment := false
	if i := slices.Index(rest, '#'); i >= 0 {
		rest, fragment, hasFragment = rest[:i], rest[i+1:], true
	}
	for len(rest) > 0 && (rest[0] == '/' || rest[0] == '\\') {
		rest = rest[1:]
	}

	authEnd := slices.IndexFunc(rest, func(c rune) bool { return c == '/' || c == '\\' || c == '?' })
	if authEnd < 0 {
		authEnd = len(rest)
	}
	auth := rest[:authEnd]
	rest = rest[authEnd:]
	out := []byte(scheme + "://")
	at := -1
	for i, c := range auth {
		if c == '@' {
			at = i
		}
	}
	if at >= 0 {
		var user, pass []byte
		password := false
		for _, c := range auth[:at] {
			field := &user
			if password {
				field = &pass
			}
			switch {
			case c == '@':
				*field = append(*field, "%40"...)
			case c == ':' && !password:
				password = true
			default:
				*field = percentEncode(*field, c, userinfoSet)
			}
		}
		auth = auth[at+1:]
		if len(user) > 0 || len(pass) > 0 {
			out = append(out, user...)
			if len(pass) > 0 {
				out = append(append(out, ':'), pass...)
			}
			out = append(out, '@')
		}
	}

	hostEnd := len(auth)
	brackets := false
	for i, c := range auth {
		if c == '[' {
			brackets = true
		} else if c == ']' {
			brackets = false
		} else if c == ':' && !brackets {
			hostEnd = i
			break
		}
	}
	if hostEnd == 0 {
		return "", false
	}
	host, ok := parseHost(auth[:hostEnd])
	if !ok {
		return "", false
	}
	out = append(out, host...)
	if hostEnd < len(auth) {
		digits := auth[hostEnd+1:]
		n := 0
		for _, c := range digits {
			if c < '0' || c > '9' {
				return "", false
			}
			n = min(n*10+int(c-'0'), 1<<16)
		}
		if n >= 1<<16 {
			return "", false
		}
		if len(digits) > 0 && n != port {
			out = strconv.AppendInt(append(out, ':'), int64(n), 10)
		}
	}

	var query []rune
	hasQuery := false
	if i := slices.Index(rest, '?'); i >= 0 {
		rest, query, hasQuery = rest[:i], rest[i+1:], true
	}
	if len(rest) > 0 {
		rest = rest[1:]
	}
	var path [][]byte
	for {
		i := slices.IndexFunc(rest, func(c rune) bool { return c == '/' || c == '\\' })
		segment := rest
		if i >= 0 {
			segment, rest = rest[:i], rest[i+1:]
		}
		var enc []byte
		for _, c := range segment {
			enc = percentEncode(enc, c, pathSet)
		}
		switch dots(enc) {
		case 2:
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
			if i < 0 {
				path = append(path, nil)
			}
		case 1:
			if i < 0 {
				path = append(path, nil)
			}
		default:
			path = append(path, enc)
		}
		if i < 0 {
			break
		}
	}
	for _, segment := range path {
		out = append(append(out, '/'), segment...)
	}
	if hasQuery {
		out = append(out, '?')
		for _, c := range query {
			out = percentEncode(out, c, specialQuerySet)
		}
	}
	if hasFragment {
		out = append(out, '#')
		for _, c := range fragment {
			out = percentEncode(out, c, fragmentSet)
		}
	}
	return string(out), true
}

// A percent-encode set, as a bit per ASCII code point; it holds every other one too.
type encodeSet [2]uint64

func (set *encodeSet) has(c rune) bool { return c >= 0x80 || set[c>>6]&(1<<(c&63)) != 0 }

func newEncodeSet(chars string) encodeSet {
	var set encodeSet
	for c := 0; c < 0x80; c++ {
		if c < 0x20 || c == 0x7f || strings.IndexByte(chars, byte(c)) >= 0 {
			set[c>>6] |= 1 << (c & 63)
		}
	}
	return set
}

// The sets of ada, which predate `^` joining the path set.
var (
	fragmentSet     = newEncodeSet(" \"<>`")
	specialQuerySet = newEncodeSet(" \"#<>'")
	pathSet         = newEncodeSet(" \"#<>?`{}")
	userinfoSet     = newEncodeSet(" \"#<>?`{}/:;=@[\\]^|")
)

func percentEncode(dst []byte, c rune, set encodeSet) []byte {
	if !set.has(c) {
		return append(dst, byte(c))
	}
	var buf [utf8.UTFMax]byte
	for _, b := range buf[:utf8.EncodeRune(buf[:], c)] {
		dst = append(dst, '%', "0123456789ABCDEF"[b>>4], "0123456789ABCDEF"[b&15])
	}
	return dst
}

// dots is 1 for a single-dot path segment, 2 for a double-dot one, and 0 otherwise.
func dots(segment []byte) int {
	s := strings.ToLower(string(segment))
	switch s {
	case ".", "%2e":
		return 1
	case "..", ".%2e", "%2e.", "%2e%2e":
		return 2
	}
	return 0
}

func parseHost(input []rune) (string, bool) {
	if input[0] == '[' {
		if input[len(input)-1] != ']' {
			return "", false
		}
		return parseIPv6(input[1 : len(input)-1])
	}
	raw := []byte(string(input))
	decoded := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] == '%' && i+2 < len(raw) && unhex(raw[i+1]) >= 0 && unhex(raw[i+2]) >= 0 {
			decoded = append(decoded, byte(unhex(raw[i+1])<<4|unhex(raw[i+2])))
			i += 2
			continue
		}
		decoded = append(decoded, raw[i])
	}
	domain, ok := domainToASCII(decoded)
	if !ok || domain == "" || strings.ContainsAny(domain, "\x7f #%/:<>?@[\\]^|") || strings.IndexFunc(domain, func(c rune) bool { return c < 0x20 }) >= 0 {
		return "", false
	}
	if endsInNumber(domain) {
		return parseIPv4(domain)
	}
	return domain, true
}

func endsInNumber(domain string) bool {
	parts := strings.Split(domain, ".")
	if parts[len(parts)-1] == "" {
		if len(parts) == 1 {
			return false
		}
		parts = parts[:len(parts)-1]
	}
	last := parts[len(parts)-1]
	if last != "" && strings.Trim(last, "0123456789") == "" {
		return true
	}
	_, ok := ipv4Number(last)
	return ok
}

// ipv4Number parses one part of an IPv4 address. Values past 32 bits all read as 1<<32.
func ipv4Number(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	radix := 10
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		s, radix = s[2:], 16
	} else if len(s) >= 2 && s[0] == '0' {
		s, radix = s[1:], 8
	}
	n := uint64(0)
	for i := 0; i < len(s); i++ {
		d := unhex(s[i])
		if d < 0 || d >= radix {
			return 0, false
		}
		n = min(n*uint64(radix)+uint64(d), 1<<32)
	}
	return n, true
}

func parseIPv4(domain string) (string, bool) {
	parts := strings.Split(domain, ".")
	if parts[len(parts)-1] == "" && len(parts) > 1 {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > 4 {
		return "", false
	}
	addr := uint64(0)
	for i, part := range parts {
		n, ok := ipv4Number(part)
		if !ok {
			return "", false
		}
		if i < len(parts)-1 {
			if n > 255 {
				return "", false
			}
			addr += n << (8 * (3 - i))
		} else {
			if n >= 1<<(8*(5-len(parts))) {
				return "", false
			}
			addr += n
		}
	}
	return strconv.Itoa(int(addr>>24)) + "." + strconv.Itoa(int(addr>>16&255)) + "." +
		strconv.Itoa(int(addr>>8&255)) + "." + strconv.Itoa(int(addr&255)), true
}

func parseIPv6(input []rune) (string, bool) {
	var addr [8]uint16
	piece, compress := 0, -1
	p := 0
	at := func(i int) rune {
		if i < len(input) {
			return input[i]
		}
		return -1
	}
	if at(0) == ':' {
		if at(1) != ':' {
			return "", false
		}
		p += 2
		piece++
		compress = piece
	}
	for p < len(input) {
		if piece == 8 {
			return "", false
		}
		if input[p] == ':' {
			if compress >= 0 {
				return "", false
			}
			p++
			piece++
			compress = piece
			continue
		}
		value, length := 0, 0
		for length < 4 && p < len(input) && input[p] < 0x80 && unhex(byte(input[p])) >= 0 {
			value = value*16 + unhex(byte(input[p]))
			p++
			length++
		}
		if at(p) == '.' {
			if length == 0 {
				return "", false
			}
			p -= length
			if piece > 6 {
				return "", false
			}
			seen := 0
			for p < len(input) {
				part := -1
				if seen > 0 {
					if input[p] != '.' || seen >= 4 {
						return "", false
					}
					p++
				}
				if c := at(p); c < '0' || c > '9' {
					return "", false
				}
				for c := at(p); c >= '0' && c <= '9'; c = at(p) {
					if part == 0 {
						return "", false
					}
					part = max(part, 0)*10 + int(c-'0')
					if part > 255 {
						return "", false
					}
					p++
				}
				addr[piece] = addr[piece]<<8 | uint16(part)
				seen++
				if seen == 2 || seen == 4 {
					piece++
				}
			}
			if seen != 4 {
				return "", false
			}
			break
		}
		if at(p) == ':' {
			p++
			if p == len(input) {
				return "", false
			}
		} else if p < len(input) {
			return "", false
		}
		addr[piece] = uint16(value)
		piece++
	}
	if compress >= 0 {
		swaps := piece - compress
		for piece = 7; piece != 0 && swaps > 0; piece, swaps = piece-1, swaps-1 {
			addr[piece], addr[compress+swaps-1] = addr[compress+swaps-1], addr[piece]
		}
	} else if piece != 8 {
		return "", false
	}

	// Compress the first longest run of two or more zero pieces.
	best, bestLen := -1, 1
	for i := 0; i < 8; {
		j := i
		for j < 8 && addr[j] == 0 {
			j++
		}
		if j-i > bestLen {
			best, bestLen = i, j-i
		}
		i = max(j, i+1)
	}
	out := []byte{'['}
	for i := 0; i < 8; i++ {
		if i == best {
			if i == 0 {
				out = append(out, ':')
			}
			out = append(out, ':')
			i += bestLen - 1
			continue
		}
		out = strconv.AppendUint(out, uint64(addr[i]), 16)
		if i < 7 {
			out = append(out, ':')
		}
	}
	return string(append(out, ']')), true
}
