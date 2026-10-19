// Package info draws Mermaid's info diagram.
package info

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

const (
	tShowInfo = iota
	tInfo
	tAccDescr
	tAccTitle
	tTitle
	tNewline
)

var tokens = []*common.TokenType{
	common.Keyword("showInfo"),
	common.Keyword("info"),
	common.ACC_DESCR,
	common.ACC_TITLE,
	common.TITLE,
	common.NEWLINE,
	common.WHITESPACE,
	common.YAML,
	common.DIRECTIVE,
	common.SINGLE_LINE_COMMENT,
}

type ast struct {
	title, accTitle, accDescr string
}

func parseInfo(src string) ast {
	r := common.NewReader(common.Tokenize(src, tokens, "info"), tokens, "info")
	var a ast
	for r.Kind() == tNewline {
		r.I++
	}
	r.Expect(tInfo)
	for r.Kind() == tNewline {
		r.I++
	}
	if r.Accept(tShowInfo) {
		for r.Kind() == tNewline {
			r.I++
		}
	}
	for r.Kind() != -1 {
		switch r.Kind() {
		case tAccDescr:
			a.accDescr = common.AccDescrValue(r.Take())
		case tAccTitle:
			a.accTitle = common.AccTitleValue(r.Take())
		case tTitle:
			a.title = common.TitleValue(r.Take())
		default:
			r.Fail("a title or the end of input")
		}
		if r.Kind() == -1 {
			break
		}
		if r.Kind() != tNewline {
			r.Fail("a line break or the end of input")
		}
		for r.Kind() == tNewline {
			r.I++
		}
	}
	return a
}

// Model is a parsed info diagram.
type Model struct {
	Title    string
	AccTitle string
	AccDescr string
}

// collapseIndents is .replace(/\n\s+/g, '\n').
func collapseIndents(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			j := i + 1
			for j < len(s) {
				r, size := js.RuneAt(s, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			b.WriteByte('\n')
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func build(a ast, title string) *Model {
	m := &Model{Title: a.title}
	if m.Title == "" {
		m.Title = title
	}
	if a.accTitle != "" {
		m.AccTitle = js.TrimStart(a.accTitle)
	}
	if a.accDescr != "" {
		m.AccDescr = collapseIndents(a.accDescr)
	}
	return m
}

func render(model *Model, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	m := options.Measurer
	pad := options.PaddingOr(8)

	title := text.Layout(model.Title, false, m, size, 4000, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 8
	}
	version := text.Layout("Pele v"+core.Version, false, m, size, 4000, 0)

	width := math.Ceil(math.Max(title.Width, version.Width) + 2*pad)
	height := math.Ceil(titleHeight + version.Height + 2*pad)
	content := svg.LabelSvg(title, width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) +
		svg.LabelSvg(version, width/2, pad+titleHeight+version.Height/2, ` class="pele-version" fill="var(--_m)"`, nil)

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{SVG: svg.Document("info", width, height, size, options, acc, content), Width: width, Height: height}
}

// Diagram is the info diagram type.
var Diagram = &core.Diagram{
	Type: "info",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return build(parseInfo(source), title)
	},
	Render: func(model any, _ *core.Object, options *core.Options) core.Rendered {
		return render(model.(*Model), options)
	},
}
