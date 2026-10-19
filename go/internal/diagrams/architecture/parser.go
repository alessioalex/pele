//go:build wip

package architecture

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tHeader = iota
	tJunction
	tService
	tColumn
	tGroup
	tAlign
	tRow
	tDashes
	tIn
	tColon
	tDash
	tDirection
	tArrowGroup
	tInto
	tAccDescr
	tAccTitle
	tTitle
	tString
	tID
	tNewline
	tWhitespace
	tYaml
	tDirective
	tComment
	tIcon
	tLabel
)

func isWordByte(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// ID is /[\w]([-\w]*\w)?/y: a run of word characters and dashes that ends on a word character.
var idToken = &common.TokenType{Name: "ID", Match: func(src string, at int) int {
	if at >= len(src) || !isWordByte(src[at]) {
		return -1
	}
	end := at + 1
	for p := end; p < len(src) && (isWordByte(src[p]) || src[p] == '-'); p++ {
		if src[p] != '-' {
			end = p + 1
		}
	}
	return end
}}

func word(text string) *common.TokenType {
	return &common.TokenType{Name: text, Literal: text, Longer: []*common.TokenType{idToken}}
}

func oneOf(chars string) common.Scanner {
	return func(src string, at int) int {
		if at < len(src) && strings.IndexByte(chars, src[at]) >= 0 {
			return at + 1
		}
		return -1
	}
}

// ARCH_ICON is /\([\w-:]+\)/y.
func iconEnd(src string, at int) int {
	if at >= len(src) || src[at] != '(' {
		return -1
	}
	p := at + 1
	for p < len(src) && (isWordByte(src[p]) || src[p] == '-' || src[p] == ':') {
		p++
	}
	if p == at+1 || p >= len(src) || src[p] != ')' {
		return -1
	}
	return p + 1
}

// ARCH_TITLE is /\[(?:"([^"\\]|\\.)*"|'([^'\\]|\\.)*'|[^\[\]\r\n]+)\]/y.
func labelEnd(src string, at int) int {
	if at >= len(src) || src[at] != '[' {
		return -1
	}
	if e := common.QuotedEnd(src, at+1); e >= 0 && e < len(src) && src[e] == ']' {
		return e + 1
	}
	p := at + 1
	for p < len(src) && src[p] != '[' && src[p] != ']' && src[p] != '\r' && src[p] != '\n' {
		p++
	}
	if p == at+1 || p >= len(src) || src[p] != ']' {
		return -1
	}
	return p + 1
}

// The direction letters come before identifiers and have no longer alternative, so Mermaid
// reads an id that starts with L, R, T or B as a direction followed by the rest.
var architectureTokens = []*common.TokenType{
	word("architecture-beta"),
	word("junction"),
	word("service"),
	word("column"),
	word("group"),
	word("align"),
	word("row"),
	{Name: "--", Literal: "--", Longer: []*common.TokenType{common.YAML}},
	word("in"),
	{Name: ":", Literal: ":"},
	{Name: "-", Literal: "-", Longer: []*common.TokenType{common.YAML}},
	{Name: "ARROW_DIRECTION", Match: oneOf("LRTB")},
	{Name: "ARROW_GROUP", Literal: "{group}"},
	{Name: "ARROW_INTO", Match: oneOf("<>")},
	common.ACC_DESCR,
	common.ACC_TITLE,
	common.TITLE,
	common.STRING,
	idToken,
	common.NEWLINE,
	common.WHITESPACE,
	common.YAML,
	common.DIRECTIVE,
	common.SINGLE_LINE_COMMENT,
	{Name: "ARCH_ICON", Match: iconEnd},
	{Name: "ARCH_TITLE", Match: labelEnd},
}

// Side is the side of a node an edge meets: L, R, T or B.
type Side = string

type groupAst struct {
	id, icon, title, in string
}

type serviceAst struct {
	id, icon, iconText, title, in string
}

type junctionAst struct {
	id, in string
}

type edgeAst struct {
	lhsID    string
	lhsGroup bool
	lhsDir   Side
	lhsInto  bool
	title    string
	rhsInto  bool
	rhsDir   Side
	rhsID    string
	rhsGroup bool
}

type alignmentAst struct {
	direction string
	members   []string
}

type architectureAst struct {
	title, accTitle, accDescr string
	groups                    []groupAst
	services                  []serviceAst
	junctions                 []junctionAst
	edges                     []edgeAst
	alignments                []alignmentAst
}

// labelValue is Mermaid's ArchitectureValueConverter for ARCH_TITLE.
func labelValue(text string) string {
	result := js.Trim(text[1 : len(text)-1])
	if (strings.HasPrefix(result, `"`) && strings.HasSuffix(result, `"`)) || (strings.HasPrefix(result, "'") && strings.HasSuffix(result, "'")) {
		if len(result) < 2 {
			result = ""
		} else {
			result = result[1 : len(result)-1]
		}
		result = strings.ReplaceAll(strings.ReplaceAll(result, `\"`, `"`), `\'`, "'")
	}
	return js.Trim(result)
}

func parseArchitecture(src string) *architectureAst {
	r := common.NewReader(common.Tokenize(src, architectureTokens, "architecture"), architectureTokens, "architecture")
	ast := &architectureAst{}

	is := func(kind int) bool { return r.Kind() == kind }
	endOfLine := func() {
		if r.Kind() == -1 {
			return
		}
		if r.Kind() != tNewline {
			r.Fail("a line break or the end of input")
		}
		for r.Kind() == tNewline {
			r.I++
		}
	}

	for r.Kind() == tNewline {
		r.I++
	}
	r.Expect(tHeader)
	for r.Kind() != -1 {
		switch r.Kind() {
		case tNewline:
			r.I++
			continue
		case tAccDescr:
			ast.accDescr = common.AccDescrValue(r.Take())
		case tAccTitle:
			ast.accTitle = common.AccTitleValue(r.Take())
		case tTitle:
			ast.title = common.TitleValue(r.Take())
		case tGroup:
			r.I++
			group := groupAst{id: r.Expect(tID)}
			if is(tIcon) {
				t := r.Take()
				group.icon = t[1 : len(t)-1]
			}
			if is(tLabel) {
				group.title = labelValue(r.Take())
			}
			if r.Accept(tIn) {
				group.in = r.Expect(tID)
			}
			ast.groups = append(ast.groups, group)
		case tService:
			r.I++
			service := serviceAst{id: r.Expect(tID)}
			if is(tString) {
				service.iconText = common.StringValue(r.Take())
			} else if is(tIcon) {
				t := r.Take()
				service.icon = t[1 : len(t)-1]
			}
			if is(tLabel) {
				service.title = labelValue(r.Take())
			}
			if r.Accept(tIn) {
				service.in = r.Expect(tID)
			}
			ast.services = append(ast.services, service)
		case tJunction:
			r.I++
			junction := junctionAst{id: r.Expect(tID)}
			if r.Accept(tIn) {
				junction.in = r.Expect(tID)
			}
			ast.junctions = append(ast.junctions, junction)
		case tID:
			e := edgeAst{lhsID: r.Take()}
			e.lhsGroup = r.Accept(tArrowGroup)
			r.Expect(tColon)
			e.lhsDir = r.Expect(tDirection)
			e.lhsInto = r.Accept(tInto)
			if !r.Accept(tDashes) {
				if !is(tDash) {
					r.Fail("'--' or '-'")
				}
				r.I++
				e.title = labelValue(r.Expect(tLabel))
				r.Expect(tDash)
			}
			e.rhsInto = r.Accept(tInto)
			e.rhsDir = r.Expect(tDirection)
			r.Expect(tColon)
			e.rhsID = r.Expect(tID)
			e.rhsGroup = r.Accept(tArrowGroup)
			ast.edges = append(ast.edges, e)
		case tAlign:
			r.I++
			if !is(tRow) && !is(tColumn) {
				r.Fail("'row' or 'column'")
			}
			a := alignmentAst{direction: r.Take()}
			a.members = []string{r.Expect(tID), r.Expect(tID)}
			for is(tID) {
				a.members = append(a.members, r.Take())
			}
			ast.alignments = append(ast.alignments, a)
		default:
			r.Fail("a group, a service, a junction, an edge, an alignment, a title, or a line break")
		}
		endOfLine()
	}
	return ast
}
