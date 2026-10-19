package common

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// The indented outline grammar that Mermaid's mindmap and kanban diagrams share.
// Kanban adds `@{ ... }` metadata after a node and keeps `@` out of node ids.

// Outline token types.
const (
	OutlineEnd = iota
	OutlineError
	OutlineSpaceline
	OutlineNL
	OutlineEOF
	OutlineKeyword
	OutlineSpacelist
	OutlineIcon
	OutlineClass
	OutlineNodeDStart
	OutlineNodeID
	OutlineNodeDescr
	OutlineNodeDEnd
	OutlineShapeData
)

// As the generated parser prints them. It has no name for the end of input and prints its number.
var outlineNames = []string{
	"1",
	"INVALID",
	"SPACELINE",
	"NL",
	"EOF",
	"",
	"SPACELIST",
	"ICON",
	"CLASS",
	"NODE_DSTART",
	"NODE_ID",
	"NODE_DESCR",
	"NODE_DEND",
	"SHAPE_DATA",
}

// OutlineTokenName is the name the generated parser gives a token type.
func OutlineTokenName(typ int, kanban bool) string {
	if typ == OutlineKeyword {
		if kanban {
			return "KANBAN"
		}
		return "MINDMAP"
	}
	return outlineNames[typ]
}

// Node types (NODE_TYPE).
const (
	NodeDefault     = 0
	NodeNoBorder    = 0
	NodeRoundedRect = 1
	NodeRect        = 2
	NodeCircle      = 3
	NodeCloud       = 4
	NodeBang        = 5
	NodeHexagon     = 6
)

// GetType is the node type for a pair of delimiters. Any opening delimiter pairs with any
// closing one, as in Mermaid.
func GetType(start, end string) int {
	switch start {
	case "[":
		return NodeRect
	case "(":
		if end == ")" {
			return NodeRoundedRect
		}
		return NodeCloud
	case "((":
		return NodeCircle
	case ")":
		return NodeCloud
	case "))":
		return NodeBang
	case "{{":
		return NodeHexagon
	default:
		return NodeDefault
	}
}

// Decoration is what an `::icon(...)` or `:::class` line adds to the last node. The tokens
// that give them are never empty, so "" is a field left out.
type Decoration struct {
	Icon  string
	Class string
}

// OutlineDb receives the grammar's actions. Levels count UTF-16 code units of indentation;
// shapeData is nil when the node has no `@{ ... }`.
type OutlineDb interface {
	GetType(start, end string) int
	AddNode(level int, id, descr string, typ int, shapeData *string)
	DecorateNode(decoration Decoration)
}

const (
	osInitial = iota
	osClass
	osIcon
	osNode
	osNstr
	osNstr2
	osData
	osDataStr
)

func outlineByte(s string, i int) int {
	if i < 0 || i >= len(s) {
		return -1
	}
	return int(s[i])
}

// lineTerminatorAt reports whether a line terminator (\n, \r, U+2028, U+2029) starts at i.
func lineTerminatorAt(s string, i int) bool {
	c := s[i]
	return c == '\n' || c == '\r' || (c == 0xe2 && i+2 < len(s) && s[i+1] == 0x80 && (s[i+2] == 0xa8 || s[i+2] == 0xa9))
}

// lineRun is /.+/y at p: the end of the run, or -1 when it is empty.
func lineRun(s string, p int) int {
	e := p
	for e < len(s) && !lineTerminatorAt(s, e) {
		e++
	}
	if e == p {
		return -1
	}
	return e
}

// runOutside is a negated ASCII class with `+`, sticky at p: the end of the run, or -1.
func runOutside(s string, p int, stops string) int {
	e := p
	for e < len(s) && strings.IndexByte(stops, s[e]) < 0 {
		e++
	}
	if e == p {
		return -1
	}
	return e
}

// replaceBreaks is .replace(/\n\s*/g, '<br/>').
func replaceBreaks(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			b.WriteString("<br/>")
			i = skipJSSpace(s, i+1)
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// TokenizeOutline reads outline text into tokens. Starts are byte offsets.
func TokenizeOutline(src string, kanban bool) ScanTokens {
	n := len(src)
	var types []int
	var texts []string
	var starts []int
	keyword := "mindmap"
	idStops := "([\n){}"
	if kanban {
		keyword = "kanban"
		idStops = "([\n){}@"
	}
	state := osInitial
	p := 0
	nextBreak := -1

	put := func(typ int, text string, end int) {
		types = append(types, typ)
		texts = append(texts, text)
		starts = append(starts, p)
		p = end
	}
	emit := func(typ, end, next int) {
		put(typ, src[p:end], end)
		state = next
	}
	upTo := func(ch byte) int {
		if e := strings.IndexByte(src[p:], ch); e >= 0 {
			return p + e
		}
		return n
	}

loop:
	for p < n {
		c := outlineByte(src, p)
		d := outlineByte(src, p+1)
		switch state {
		case osInitial:
			// Only characters at or below a space, or from U+00A0 on, can start a `\s` run.
			w := skipJSSpace(src, p)
			if kanban && c == '@' && d == '{' {
				put(OutlineShapeData, "", p+2)
				state = osData
			} else if outlineByte(src, w) == '%' && outlineByte(src, w+1) == '%' {
				emit(OutlineSpaceline, lineRun(src, w), osInitial)
			} else if w > p {
				if nextBreak < p {
					nextBreak = upTo('\n')
				}
				if nextBreak >= w {
					emit(OutlineSpacelist, w, osInitial)
				} else if e := strings.LastIndexByte(src[:w], '\n'); e > p {
					emit(OutlineSpaceline, e+1, osInitial)
				} else {
					emit(OutlineNL, p+1, osInitial)
				}
			} else if KeywordAt(src, p, keyword) && !js.IsWord(rune(outlineByte(src, p+len(keyword)))) {
				emit(OutlineKeyword, p+len(keyword), osInitial)
			} else if c == ':' && d == ':' && outlineByte(src, p+2) == ':' {
				p += 3
				state = osClass
			} else if c == ':' && d == ':' && KeywordAt(src, p+2, "icon") && outlineByte(src, p+6) == '(' {
				p += 7
				state = osIcon
			} else if (c == '-' && d == ')') || (c == '(' && (d == '-' || d == '(')) || ((c == ')' || c == '{') && d == c) {
				emit(OutlineNodeDStart, p+2, osNode)
			} else if c == ')' || c == '(' || c == '[' {
				emit(OutlineNodeDStart, p+1, osNode)
			} else if e := runOutside(src, p, idStops); e != -1 {
				emit(OutlineNodeID, e, osInitial)
			} else {
				break loop
			}
		case osNode:
			if c == '"' {
				if d == '`' {
					state = osNstr2
					p += 2
				} else {
					state = osNstr
					p++
				}
			} else if c == ')' || c == '(' {
				end := p + 1
				if d == c || (c == '(' && d == '-') {
					end = p + 2
				}
				emit(OutlineNodeDEnd, end, osInitial)
			} else if c == ']' {
				emit(OutlineNodeDEnd, p+1, osInitial)
			} else if (c == '}' && d == '}') || (c == '-' && d == ')') {
				emit(OutlineNodeDEnd, p+2, osInitial)
			} else if c == '}' {
				// A lone closing brace takes the rest of its line as text.
				emit(OutlineNodeDescr, lineRun(src, p), osNode)
			} else {
				emit(OutlineNodeDescr, runOutside(src, p, ")](}"), osNode)
			}
		case osNstr:
			if c == '"' {
				p++
				state = osNode
			} else {
				emit(OutlineNodeDescr, upTo('"'), osNstr)
			}
		case osNstr2:
			if c == '`' && d == '"' {
				p += 2
				state = osNode
			} else if e := runOutside(src, p, "`\""); e != -1 {
				emit(OutlineNodeDescr, e, osNstr2)
			} else {
				break loop
			}
		case osClass:
			if e := lineRun(src, p); e != -1 {
				emit(OutlineClass, e, osInitial)
			} else if c == '\n' {
				p++
				state = osInitial
			} else {
				break loop
			}
		case osIcon:
			if c == ')' {
				p++
				state = osInitial
			} else {
				emit(OutlineIcon, upTo(')'), osIcon)
			}
		case osData:
			if c == '"' {
				emit(OutlineShapeData, p+1, osDataStr)
			} else if c == '}' {
				p++
				state = osInitial
			} else if e := runOutside(src, p, "}^\""); e != -1 {
				emit(OutlineShapeData, e, osData)
			} else {
				break loop
			}
		default:
			if c == '"' {
				emit(OutlineShapeData, p+1, osData)
			} else {
				e := upTo('"')
				put(OutlineShapeData, replaceBreaks(src[p:e]), e)
			}
		}
	}

	if p < n {
		put(OutlineError, "", p)
	} else {
		if state == osInitial {
			put(OutlineEOF, "", n)
		}
		put(OutlineEnd, "", n)
	}
	return ScanTokens{Types: types, Texts: texts, Starts: starts}
}

const outlineStop = "'SPACELINE', 'NL', 'EOF'"

func outlineStart(t int) bool {
	switch t {
	case OutlineSpaceline, OutlineSpacelist, OutlineIcon, OutlineClass, OutlineNodeDStart, OutlineNodeID:
		return true
	}
	return false
}

func outlineIsStop(t int) bool {
	return t == OutlineSpaceline || t == OutlineNL || t == OutlineEOF
}

// ParseOutline makes the calls on db that the grammar's actions make, in the same order, and
// reports errors in the state where the generated parser would.
func ParseOutline(src string, db OutlineDb, kanban bool) {
	diagram := "mindmap"
	data := ""
	if kanban {
		diagram = "kanban"
		data = ", 'SHAPE_DATA'"
	}
	tok := TokenizeOutline(src, kanban)
	types, texts, starts := tok.Types, tok.Texts, tok.Starts
	i := 0

	fail := func(expected string) {
		t := types[i]
		if t == OutlineError {
			panic(core.SyntaxError(diagram, src, starts[i], "", true))
		}
		panic(core.SyntaxError(diagram, src, starts[i], "Expecting "+expected+", got '"+OutlineTokenName(t, kanban)+"'", false))
	}

	keyword := "'" + OutlineTokenName(OutlineKeyword, kanban) + "'"
	if types[i] == OutlineSpaceline {
		i++
		for types[i] == OutlineSpaceline || types[i] == OutlineNL {
			i++
		}
		if types[i] != OutlineKeyword {
			fail("'SPACELINE', 'NL', " + keyword)
		}
	} else if types[i] != OutlineKeyword {
		fail("'SPACELINE', " + keyword)
	}
	i++
	first := "'SPACELINE', 'NL', 'SPACELIST', 'ICON', 'CLASS', 'NODE_DSTART', 'NODE_ID'"
	if types[i] == OutlineNL {
		i++
		first = "'SPACELINE', 'SPACELIST', 'ICON', 'CLASS', 'NODE_DSTART', 'NODE_ID'"
	}

	for {
		t := types[i]
		level := 0
		indented := t == OutlineSpacelist
		if indented {
			level = js.Len(texts[i])
			i++
			t = types[i]
		}
		if t == OutlineNodeID || t == OutlineNodeDStart {
			var id, descr string
			hasID := false
			typ := 0
			if t == OutlineNodeID {
				id, hasID = texts[i], true
				i++
			}
			if types[i] == OutlineNodeDStart {
				start := texts[i]
				i++
				if types[i] != OutlineNodeDescr {
					fail("'NODE_DESCR'")
				}
				descr = texts[i]
				i++
				if types[i] != OutlineNodeDEnd {
					fail("'NODE_DEND'")
				}
				end := texts[i]
				i++
				if !outlineIsStop(types[i]) && types[i] != OutlineShapeData {
					fail(outlineStop + data)
				}
				typ = db.GetType(start, end)
			} else {
				if !outlineIsStop(types[i]) && types[i] != OutlineShapeData {
					fail(outlineStop + ", 'NODE_DSTART'" + data)
				}
				descr = id
			}
			if !hasID {
				id = descr
			}
			if types[i] == OutlineShapeData {
				var shapeData strings.Builder
				for types[i] == OutlineShapeData {
					shapeData.WriteString(texts[i])
					i++
				}
				if !outlineIsStop(types[i]) {
					fail(outlineStop + data)
				}
				s := shapeData.String()
				db.AddNode(level, id, descr, typ, &s)
			} else {
				db.AddNode(level, id, descr, typ, nil)
			}
		} else if t == OutlineIcon || t == OutlineClass {
			text := texts[i]
			i++
			if !outlineIsStop(types[i]) {
				fail(outlineStop)
			}
			if t == OutlineIcon {
				db.DecorateNode(Decoration{Icon: text})
			} else {
				db.DecorateNode(Decoration{Class: text})
			}
		} else if indented {
			if !outlineIsStop(t) {
				fail(outlineStop + ", 'ICON', 'CLASS', 'NODE_DSTART', 'NODE_ID'")
			}
		} else if t == OutlineSpaceline {
			i++
			if !outlineIsStop(types[i]) {
				fail(outlineStop)
			}
		} else {
			fail(first)
		}

		i++
		for types[i] == OutlineNL || types[i] == OutlineEOF {
			i++
		}
		if types[i] == OutlineEnd {
			return
		}
		if !outlineStart(types[i]) {
			fail(outlineStop + ", 'SPACELIST', 'ICON', 'CLASS', 'NODE_DSTART', 'NODE_ID'")
		}
	}
}
