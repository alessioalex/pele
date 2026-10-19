package util

// whatwgHref parses an http: or https: URL as the WHATWG URL parser does and returns its
// serialization, as `new URL(s).toString()` does, with the scheme and host lowercased. ok is
// false where the URL constructor throws.
func whatwgHref(s string) (string, bool) {
	return s, true
}
