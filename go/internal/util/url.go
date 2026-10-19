// Package util holds helpers shared by the diagram types: URL checks and Unicode tables.
package util

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

const blank = "about:blank"

func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// DecodeURIComponent is decodeURIComponent; ok is false where it throws a URIError.
func DecodeURIComponent(s string) (string, bool) {
	if !strings.Contains(s, "%") {
		return s, true
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '%' {
			b.WriteByte(s[i])
			i++
			continue
		}
		read := func(at int) (byte, bool) {
			if at+3 > len(s) || s[at] != '%' {
				return 0, false
			}
			h, l := unhex(s[at+1]), unhex(s[at+2])
			if h < 0 || l < 0 {
				return 0, false
			}
			return byte(h<<4 | l), true
		}
		first, ok := read(i)
		if !ok {
			return s, false
		}
		if first < 0x80 {
			b.WriteByte(first)
			i += 3
			continue
		}
		n := 0
		switch {
		case first&0xe0 == 0xc0:
			n = 2
		case first&0xf0 == 0xe0:
			n = 3
		case first&0xf8 == 0xf0:
			n = 4
		default:
			return s, false
		}
		seq := []byte{first}
		for k := 1; k < n; k++ {
			c, ok := read(i + 3*k)
			if !ok || c&0xc0 != 0x80 {
				return s, false
			}
			seq = append(seq, c)
		}
		r, size := utf8.DecodeRune(seq)
		if r == utf8.RuneError || size != n {
			return s, false
		}
		b.Write(seq)
		i += 3 * n
	}
	return b.String(), true
}

func decode(uri string) string {
	if d, ok := DecodeURIComponent(uri); ok {
		return d
	}
	return uri
}

// isCtrl is RE_CTRL: /[\u0000-\u001F\u007F-\u009F -‍\uFEFF]/.
func isCtrl(r rune) bool {
	return r <= 0x1f || (r >= 0x7f && r <= 0x9f) || (r >= 0x2000 && r <= 0x200d) || r == 0xfeff
}

func hasCtrl(s string) bool { return strings.IndexFunc(s, isCtrl) >= 0 }

func dropCtrl(s string) string {
	if !hasCtrl(s) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isCtrl(r) {
			return -1
		}
		return r
	}, s)
}

// htmlEntities replaces /&#(\w+)(^\w|;)?/g with String.fromCharCode of the digits, and reports
// whether there was any.
func htmlEntities(s string) (string, bool) {
	if !strings.Contains(s, "&#") {
		return s, false
	}
	var b strings.Builder
	found := false
	last := 0
	for i := 0; i+2 < len(s); i++ {
		if s[i] != '&' || s[i+1] != '#' || !js.IsWord(rune(s[i+2])) {
			continue
		}
		j := i + 2
		for j < len(s) && js.IsWord(rune(s[j])) {
			j++
		}
		code := js.ToUint32(js.ToNumber(s[i+2:j])) & 0xffff
		end := j
		if end < len(s) && s[end] == ';' {
			end++
		}
		b.WriteString(s[last:i])
		b.WriteString(string(rune(code)))
		last = end
		i = end - 1
		found = true
	}
	if !found {
		return s, false
	}
	b.WriteString(s[last:])
	return b.String(), true
}

// htmlCtrlEntities removes /&(newline|tab);/gi.
func htmlCtrlEntities(s string) (string, bool) {
	if !strings.Contains(s, "&") {
		return s, false
	}
	var b strings.Builder
	found := false
	last := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '&' {
			continue
		}
		for _, name := range []string{"newline;", "tab;"} {
			if js.HasPrefixFold(s[i+1:], name) {
				b.WriteString(s[last:i])
				last = i + 1 + len(name)
				i = last - 1
				found = true
				break
			}
		}
	}
	if !found {
		return s, false
	}
	b.WriteString(s[last:])
	return b.String(), true
}

// wsEscapes removes /(\\|%5[cC])((%(6[eE]|72|74))|[nrt])/g.
func wsEscapes(s string) (string, bool) {
	var b strings.Builder
	found := false
	last := 0
	for i := 0; i < len(s); i++ {
		n := 0
		if s[i] == '\\' {
			n = 1
		} else if s[i] == '%' && i+2 < len(s) && s[i+1] == '5' && (s[i+2] == 'c' || s[i+2] == 'C') {
			n = 3
		}
		if n == 0 {
			continue
		}
		rest := s[i+n:]
		m := 0
		if len(rest) >= 3 && rest[0] == '%' && ((rest[1] == '6' && (rest[2] == 'e' || rest[2] == 'E')) || (rest[1] == '7' && (rest[2] == '2' || rest[2] == '4'))) {
			m = 3
		} else if len(rest) >= 1 && (rest[0] == 'n' || rest[0] == 'r' || rest[0] == 't') {
			m = 1
		}
		if m == 0 {
			continue
		}
		b.WriteString(s[last:i])
		last = i + n + m
		i = last - 1
		found = true
	}
	if !found {
		return s, false
	}
	b.WriteString(s[last:])
	return b.String(), true
}

// schemeMatch is the first match of /^.+(:|&colon;)/gim.
func schemeMatch(s string) (string, bool) {
	start := 0
	for start <= len(s) {
		end := start
		for end < len(s) {
			r, size := js.RuneAt(s, end)
			if js.IsLineEnd(r) {
				break
			}
			end += size
		}
		line := s[start:end]
		// `.+` is greedy: the match ends at the last separator that has a character before it.
		best := -1
		for p := len(line) - 1; p >= 1; p-- {
			if line[p] == ':' {
				best = p + 1
				break
			}
			if strings.HasPrefix(line[p:], "&colon;") {
				best = p + len("&colon;")
				break
			}
		}
		if best > 0 {
			return line[:best], true
		}
		if end >= len(s) {
			break
		}
		_, size := js.RuneAt(s, end)
		start = end + size
	}
	return "", false
}

// invalidProtocol is /^([^\w]*)(javascript|data|vbscript)/im, for a protocol of one line.
func invalidProtocol(protocol string) bool {
	lower := js.ASCIILower(protocol)
	i := 0
	for i < len(lower) && !js.IsWord(rune(lower[i])) {
		i++
	}
	rest := lower[i:]
	return strings.HasPrefix(rest, "javascript") || strings.HasPrefix(rest, "data") || strings.HasPrefix(rest, "vbscript")
}

// SanitizeURL follows the algorithm of @braintree/sanitize-url, which Mermaid applies to click
// targets.
func SanitizeURL(url string) string {
	if url == "" {
		return blank
	}
	decoded := decode(js.Trim(url))
	rounds := 0
	for {
		// Each round peels one layer of encoding. A URL wrapped this many times is not a real one.
		rounds++
		if rounds > 32 {
			return blank
		}
		decoded = dropCtrl(decoded)
		decoded, _ = htmlEntities(decoded)
		decoded, _ = htmlCtrlEntities(decoded)
		decoded = dropCtrl(decoded)
		decoded, _ = wsEscapes(decoded)
		decoded = js.Trim(decoded)
		decoded = decode(decoded)
		_, ent := htmlEntities(decoded)
		_, ctrlEnt := htmlCtrlEntities(decoded)
		_, ws := wsEscapes(decoded)
		if !(hasCtrl(decoded) || ent || ctrlEnt || ws) {
			break
		}
	}
	if decoded == "" {
		return blank
	}
	if decoded[0] == '.' || decoded[0] == '/' {
		return decoded
	}
	trimmed := js.TrimStart(decoded)
	scheme, ok := schemeMatch(trimmed)
	if !ok {
		return decoded
	}
	protocol := js.Trim(js.ToLower(scheme))
	if invalidProtocol(protocol) {
		return blank
	}
	normalized := strings.ReplaceAll(trimmed, "\\", "/")
	if protocol == "mailto:" || strings.Contains(protocol, "://") {
		return normalized
	}
	if protocol == "http:" || protocol == "https:" {
		href, ok := whatwgHref(normalized)
		if !ok {
			return blank
		}
		return href
	}
	return normalized
}

// SafeURL is a URL as written in a diagram: entity codes are read first, so they cannot hide a
// scheme.
func SafeURL(url string) string { return SanitizeURL(text.DecodeEntities(url)) }

var (
	linkSchemes  = []string{"http", "https", "mailto", "tel"}
	imageSchemes = []string{"http", "https"}
)

// schemeName is /^([a-z][a-z0-9+.-]*):/i.
func schemeName(url string) (string, bool) {
	if url == "" || !((url[0] >= 'a' && url[0] <= 'z') || (url[0] >= 'A' && url[0] <= 'Z')) {
		return "", false
	}
	i := 1
	for i < len(url) {
		c := url[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '.' || c == '-' {
			i++
			continue
		}
		break
	}
	if i < len(url) && url[i] == ':' {
		return url[:i], true
	}
	return "", false
}

// Stricter than Mermaid, which lets any scheme but three through: a diagram in a shared note
// could otherwise open a local file, a network share, or another application.
func allow(url string, schemes []string) (string, bool) {
	if len(url) >= 2 && (url[0] == '/' || url[0] == '\\') && (url[1] == '/' || url[1] == '\\') {
		return "", false
	}
	scheme, ok := schemeName(url)
	if !ok || slices.Contains(schemes, js.ToLower(scheme)) {
		return url, true
	}
	return "", false
}

// LinkURL takes a URL that SanitizeURL has passed and keeps it only when it is relative or its
// scheme is one the host allows. One that is refused is left out of the drawing: written as
// `about:blank`, a link would still take the page it is in somewhere when clicked.
func LinkURL(url string, options *core.Options) (string, bool) {
	if options.Links != nil && !*options.Links {
		return "", false
	}
	schemes := options.LinkSchemes
	if schemes == nil {
		schemes = linkSchemes
	}
	return allow(url, schemes)
}

// ImageURL is LinkURL for images.
func ImageURL(url string, options *core.Options) (string, bool) {
	if options.Images != nil && !*options.Images {
		return "", false
	}
	schemes := options.ImageSchemes
	if schemes == nil {
		schemes = imageSchemes
	}
	return allow(url, schemes)
}

// Linked wraps body in a link to url when the host allows it, and records the link.
func Linked(body, url, id string, links *[]core.LinkInfo, options *core.Options, attrs string) string {
	href, ok := LinkURL(url, options)
	if !ok {
		return body
	}
	*links = append(*links, core.LinkInfo{ID: id, Href: href})
	return `<a href="` + svg.Esc(href) + `"` + attrs + RelAttr(options) + ">" + body + "</a>"
}

// RelAttr writes the link `rel` attribute.
func RelAttr(options *core.Options) string {
	rel := "noopener"
	if options.LinkRel != nil {
		rel = *options.LinkRel
	}
	if rel == "" {
		return ""
	}
	return ` rel="` + svg.Esc(rel) + `"`
}
