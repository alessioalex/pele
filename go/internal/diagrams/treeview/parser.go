package treeview

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tKeyword = iota
	tAccDescr
	tAccTitle
	tTitle
	tClassAnnotation
	tIconAnnotation
	tDescAnnotation
	tIndentation
	tQuotedName
	tWS
	tComment
	tNewline
	tBareName
)

func isBlank(c byte) bool { return c == ' ' || c == '\t' }

func byteAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func skipBlanks(src string, i int) int {
	for i < len(src) && isBlank(src[i]) {
		i++
	}
	return i
}

func annotationAt(src string, at int) bool {
	rest := src[at:]
	if strings.HasPrefix(rest, "##") || strings.HasPrefix(rest, "icon(") {
		return true
	}
	if !strings.HasPrefix(rest, ":::") {
		return false
	}
	c := byteAt(src, skipBlanks(src, at+3))
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

// matchBareName looks for an annotation once per run of blanks, where the pattern would look
// ahead from every blank.
func matchBareName(src string, at int) int {
	first := src[at]
	if isBlank(first) || first == '\n' || first == '\r' || first == '"' || first == '\'' {
		return -1
	}
	rest := src[at:]
	if strings.HasPrefix(rest, ":::") || strings.HasPrefix(rest, "icon(") || strings.HasPrefix(rest, "##") {
		return -1
	}
	n := len(src)
	i := at + 1
	for i < n {
		c := src[i]
		if c == '\n' || c == '\r' {
			break
		}
		if isBlank(c) {
			j := skipBlanks(src, i+1)
			if annotationAt(src, j) {
				break
			}
			i = j
		} else {
			i++
		}
	}
	return i
}

func isWordByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func skipWordDash(src string, i int) int {
	for i < len(src) && (isWordByte(src[i]) || src[i] == '-') {
		i++
	}
	return i
}

var bareName = &common.TokenType{Name: "BARE_NAME", Match: matchBareName}

func keywordLonger() *common.TokenType {
	k := common.Keyword("treeView-beta")
	k.Longer = []*common.TokenType{bareName}
	return k
}

var tokens = []*common.TokenType{
	keywordLonger(),
	common.AccDescr("ACC_DESCR"),
	common.ACC_TITLE,
	common.TITLE,
	// /[ \t]+:::[ \t]*[A-Za-z_][\w-]*/y
	{Name: "CLASS_ANNOTATION", First: " \t", Match: func(src string, at int) int {
		p := skipBlanks(src, at)
		if p == at || !strings.HasPrefix(src[p:], ":::") {
			return -1
		}
		p = skipBlanks(src, p+3)
		c := byteAt(src, p)
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
			return -1
		}
		return skipWordDash(src, p+1)
	}},
	// /[ \t]+icon\([\w-]*(?::[\w-]+)?\)/y
	{Name: "ICON_ANNOTATION", First: " \t", Match: func(src string, at int) int {
		p := skipBlanks(src, at)
		if p == at || !strings.HasPrefix(src[p:], "icon(") {
			return -1
		}
		p = skipWordDash(src, p+5)
		if byteAt(src, p) == ':' {
			if q := skipWordDash(src, p+1); q > p+1 && byteAt(src, q) == ')' {
				return q + 1
			}
		}
		if byteAt(src, p) == ')' {
			return p + 1
		}
		return -1
	}},
	// /[ \t]+##[^\n\r]*/y
	{Name: "DESC_ANNOTATION", First: " \t", Match: func(src string, at int) int {
		p := skipBlanks(src, at)
		if p == at || !strings.HasPrefix(src[p:], "##") {
			return -1
		}
		p += 2
		for p < len(src) && src[p] != '\n' && src[p] != '\r' {
			p++
		}
		return p
	}},
	{Name: "INDENTATION", First: " \t", Match: func(src string, at int) int {
		if p := skipBlanks(src, at); p > at {
			return p
		}
		return -1
	}},
	// /"[^"]*"|'[^']*'/y
	{Name: "QUOTED_NAME", First: `"'`, Match: func(src string, at int) int {
		q := src[at]
		if q != '"' && q != '\'' {
			return -1
		}
		if end := strings.IndexByte(src[at+1:], q); end >= 0 {
			return at + 1 + end + 1
		}
		return -1
	}},
	{Name: "WS", First: " \t", Hidden: true, Match: func(src string, at int) int {
		if p := skipBlanks(src, at); p > at {
			return p
		}
		return -1
	}},
	// /\%\%[^\n]*/y
	{Name: "ML_COMMENT", First: "%", Hidden: true, Match: func(src string, at int) int {
		if !strings.HasPrefix(src[at:], "%%") {
			return -1
		}
		if end := strings.IndexByte(src[at:], '\n'); end >= 0 {
			return at + end
		}
		return len(src)
	}},
	{Name: "NL", First: "\r\n", Hidden: true, Match: common.NewlineEnd},
	bareName,
}

type nodeAST struct {
	// indent is -1 when the node has no indentation.
	indent          int
	name            string
	classAnnotation *string
	iconAnnotation  *string
	descAnnotation  *string
}

type treeAST struct {
	title, accTitle, accDescr string
	nodes                     []nodeAST
}

func trimBlanksEnd(text string) string {
	end := len(text)
	for end > 0 && isBlank(text[end-1]) {
		end--
	}
	return text[:end]
}

func parseTreeView(src string) treeAST {
	r := common.NewReader(common.Tokenize(src, tokens, "treeView"), tokens, "treeView")
	var ast treeAST
	r.Expect(tKeyword)
	for {
		if r.Kind() == tAccDescr {
			ast.accDescr = common.AccDescrValue(r.Take())
		} else if r.Kind() == tAccTitle {
			ast.accTitle = common.AccTitleValue(r.Take())
		} else if r.Kind() == tTitle {
			ast.title = common.TitleValue(r.Take())
		} else {
			break
		}
	}
	for r.Kind() != -1 {
		node := nodeAST{indent: -1}
		if r.Kind() == tIndentation {
			node.indent = len(r.Take())
		}
		if r.Kind() == tQuotedName {
			t := r.Take()
			node.name = t[1 : len(t)-1]
		} else if r.Kind() == tBareName {
			node.name = trimBlanksEnd(r.Take())
		} else {
			r.Fail("a name")
		}
		for {
			if r.Kind() == tClassAnnotation {
				v := js.Trim(js.Trim(r.Take())[3:])
				node.classAnnotation = &v
			} else if r.Kind() == tIconAnnotation {
				t := js.Trim(r.Take())
				v := t[5 : len(t)-1]
				node.iconAnnotation = &v
			} else if r.Kind() == tDescAnnotation {
				v := js.Trim(js.Trim(r.Take())[2:])
				node.descAnnotation = &v
			} else {
				break
			}
		}
		ast.nodes = append(ast.nodes, node)
	}
	return ast
}
