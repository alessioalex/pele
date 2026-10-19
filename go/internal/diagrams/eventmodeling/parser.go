package eventmodeling

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

// EM_ID is /[_a-zA-Z][\w_]*/y.
var emID = &common.TokenType{Name: "EM_ID", Match: func(src string, at int) int {
	if at >= len(src) {
		return -1
	}
	c := rune(src[at])
	if c != '_' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') {
		return -1
	}
	p := at + 1
	for p < len(src) && js.IsWord(rune(src[p])) {
		p++
	}
	return p
}}

// Langium gives every keyword that is also an identifier the identifier as a longer alternative.
func word(text string) *common.TokenType {
	return &common.TokenType{Name: text, Literal: text, Longer: []*common.TokenType{emID}}
}

func sign(text string) *common.TokenType { return &common.TokenType{Name: text, Literal: text} }

func renamed(t *common.TokenType, name string) *common.TokenType {
	c := *t
	c.Name = name
	return &c
}

var words = []string{
	"resetframe", "readmodel", "processor", "timeframe", "command", "entity", "event", "jsobj", "figma", "given", "json", "salt", "html",
	"text", "data", "note", "when", "then", "rmo", "cmd", "evt", "pcr", "uri",
}

// lineStop is the first JavaScript line terminator at or after p, or len(src).
func lineStop(src string, p int) int {
	for ; p < len(src); p++ {
		switch src[p] {
		case '\n', '\r':
			return p
		case 0xe2:
			if p+2 < len(src) && src[p+1] == 0x80 && (src[p+2] == 0xa8 || src[p+2] == 0xa9) {
				return p
			}
		}
	}
	return len(src)
}

// matchInline is /\{(.*)\}|"(.*)"|'(.*)'/y.
func matchInline(src string, at int) int {
	if at >= len(src) {
		return -1
	}
	closer := src[at]
	switch closer {
	case '{':
		closer = '}'
	case '"', '\'':
	default:
		return -1
	}
	stop := lineStop(src, at+1)
	i := strings.LastIndexByte(src[at+1:stop], closer)
	if i < 0 {
		return -1
	}
	return at + 1 + i + 1
}

// fenceTail is (?:\r?\n|(?!\S)) at p, returning its end or -1.
func fenceTail(src string, p int) int {
	if e := common.NewlineEnd(src, p); e >= 0 {
		return e
	}
	r, _ := js.RuneAt(src, p)
	if p >= len(src) || js.IsSpace(r) {
		return p
	}
	return -1
}

// matchBlock is /\{[\t ]*\r?\n(?:[\S\s]*?\r?\n)?\}(?:\r?\n|(?!\S))/y.
func matchBlock(src string, at int) int {
	if at >= len(src) || src[at] != '{' {
		return -1
	}
	p := at + 1
	for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
		p++
	}
	b := common.NewlineEnd(src, p)
	if b < 0 {
		return -1
	}
	// The optional group comes first: the earliest line break that a closing brace follows.
	for k := b; k < len(src); k++ {
		i := strings.IndexByte(src[k:], '\n')
		if i < 0 {
			break
		}
		k += i
		if k+1 < len(src) && src[k+1] == '}' {
			if end := fenceTail(src, k+2); end >= 0 {
				return end
			}
		}
	}
	if b < len(src) && src[b] == '}' {
		return fenceTail(src, b+1)
	}
	return -1
}

var tokens = func() []*common.TokenType {
	kw := *common.Keyword("eventmodeling")
	kw.Longer = []*common.TokenType{emID}
	out := []*common.TokenType{&kw}
	for _, w := range words {
		out = append(out, word(w))
	}
	return append(out,
		sign("->>"),
		word("gwt"),
		word("ui"),
		word("md"),
		word("tf"),
		sign("[["),
		sign("]]"),
		word("rf"),
		sign("`"),
		sign("."),
		emID,
		&common.TokenType{Name: "EM_FID", Match: func(src string, at int) int {
			p := at
			for p < len(src) && p-at < 3 && src[p] >= '0' && src[p] <= '9' {
				p++
			}
			if p == at {
				return -1
			}
			return p
		}},
		&common.TokenType{Name: "EM_DATA_INLINE", Match: matchInline},
		&common.TokenType{Name: "EM_DATA_BLOCK", Match: matchBlock},
		common.AccDescr("EM_ACC_DESCR"),
		renamed(common.ACC_TITLE, "EM_ACC_TITLE"),
		renamed(common.TITLE, "EM_TITLE"),
		&common.TokenType{Name: "EM_WS", Hidden: true, Match: func(src string, at int) int {
			p := at
			for p < len(src) {
				r, size := js.RuneAt(src, p)
				if !js.IsSpace(r) {
					break
				}
				p += size
			}
			if p == at {
				return -1
			}
			return p
		}},
		renamed(common.YAML, "EM_YAML"),
		renamed(common.DIRECTIVE, "EM_DIRECTIVE"),
		renamed(common.SINGLE_LINE_COMMENT, "EM_SINGLE_LINE_COMMENT"),
		&common.TokenType{Name: "EM_ML_COMMENT", Hidden: true, Match: func(src string, at int) int {
			if !strings.HasPrefix(src[at:], "/*") {
				return -1
			}
			i := strings.Index(src[at+2:], "*/")
			if i < 0 {
				return -1
			}
			return at + 2 + i + 2
		}},
		&common.TokenType{Name: "EM_SL_COMMENT", Hidden: true, Match: func(src string, at int) int {
			if !strings.HasPrefix(src[at:], "//") {
				return -1
			}
			p := at + 2
			for p < len(src) && src[p] != '\n' && src[p] != '\r' {
				p++
			}
			return p
		}},
	)
}()

func index(name string) int {
	for k, t := range tokens {
		if t.Name == name {
			return k
		}
	}
	return -1
}

func kinds(names ...string) map[int]bool {
	out := map[int]bool{}
	for _, n := range names {
		out[index(n)] = true
	}
	return out
}

var (
	tKeyword  = 0
	tArrow    = index("->>")
	tOpen     = index("[[")
	tClose    = index("]]")
	tTick     = index("`")
	tDot      = index(".")
	tID       = index("EM_ID")
	tFrameID  = index("EM_FID")
	tInline   = index("EM_DATA_INLINE")
	tBlock    = index("EM_DATA_BLOCK")
	tAccDescr = index("EM_ACC_DESCR")
	tAccTitle = index("EM_ACC_TITLE")
	tTitle    = index("EM_TITLE")
	tEntity   = index("entity")
	tData     = index("data")
	tNote     = index("note")
	tGwt      = index("gwt")
	tGiven    = index("given")
	tWhen     = index("when")
	tThen     = index("then")

	timeFrame  = kinds("tf", "timeframe")
	resetFrame = kinds("rf", "resetframe")
	entityType = kinds("rmo", "readmodel", "ui", "cmd", "command", "evt", "event", "pcr", "processor")
	dataTypes  = kinds("json", "jsobj", "figma", "salt", "uri", "md", "html", "text")
)

// Cross-references are read as Mermaid reads them: by their text, never linked.
type emFrame struct {
	reset            bool
	name             string
	modelEntityType  string
	entityIdentifier string
	sourceFrames     []string
	// "" for none; the tokens these come from are never empty.
	dataReference   string
	dataType        string
	dataInlineValue string
	// Byte offset of the frame in the source, for error messages.
	offset int
}

type emDataEntity struct {
	name, dataType, dataBlockValue string
}

type emNoteEntity struct {
	sourceFrame, dataType, dataBlockValue string
}

type emGwt struct {
	sourceFrame                                     string
	givenStatements, whenStatements, thenStatements []string
}

type eventModelAst struct {
	// The whole token, keyword included: the grammar's own terminals get no value conversion.
	title, accTitle, accDescr string
	modelEntities             []string
	frames                    []emFrame
	dataEntities              []emDataEntity
	noteEntities              []emNoteEntity
	gwtEntities               []emGwt
	source                    string
}

func parseEventModel(src string) *eventModelAst {
	r := common.NewReader(common.Tokenize(src, tokens, "eventmodeling"), tokens, "eventmodeling")
	a := &eventModelAst{source: src}
	at := func(kind int) bool { return r.Kind() == kind }
	one := func(set map[int]bool, expected string) string {
		if !set[r.Kind()] {
			r.Fail(expected)
		}
		return r.Take()
	}
	qualifiedName := func() string {
		name := r.Expect(tID)
		for r.Accept(tDot) {
			name += "." + r.Expect(tID)
		}
		return name
	}
	dataType := func() string {
		if !r.Accept(tTick) {
			return ""
		}
		t := one(dataTypes, "a data type")
		r.Expect(tTick)
		return t
	}
	statements := func() []string {
		var out []string
		for {
			one(entityType, "an entity type")
			out = append(out, r.Expect(tID))
			if !entityType[r.Kind()] {
				break
			}
		}
		return out
	}

	r.Expect(tKeyword)
	for !at(-1) {
		kind := r.Kind()
		switch {
		case kind == tAccDescr:
			a.accDescr = r.Take()
		case kind == tAccTitle:
			a.accTitle = r.Take()
		case kind == tTitle:
			a.title = r.Take()
		case kind == tEntity:
			r.I++
			a.modelEntities = append(a.modelEntities, qualifiedName())
		case timeFrame[kind] || resetFrame[kind]:
			f := emFrame{reset: !timeFrame[kind], offset: r.Tokens.Starts[r.I]}
			r.I++
			f.name = r.Expect(tFrameID)
			f.modelEntityType = one(entityType, "an entity type")
			f.entityIdentifier = qualifiedName()
			for r.Accept(tArrow) {
				f.sourceFrames = append(f.sourceFrames, r.Expect(tFrameID))
			}
			if r.Accept(tOpen) {
				f.dataReference = r.Expect(tID)
				r.Expect(tClose)
			}
			if at(tTick) || at(tInline) {
				f.dataType = dataType()
				f.dataInlineValue = r.Expect(tInline)
			}
			a.frames = append(a.frames, f)
		case kind == tData:
			r.I++
			e := emDataEntity{name: r.Expect(tID)}
			e.dataType = dataType()
			e.dataBlockValue = r.Expect(tBlock)
			a.dataEntities = append(a.dataEntities, e)
		case kind == tNote:
			r.I++
			n := emNoteEntity{sourceFrame: r.Expect(tFrameID)}
			n.dataType = dataType()
			n.dataBlockValue = r.Expect(tBlock)
			a.noteEntities = append(a.noteEntities, n)
		case kind == tGwt:
			r.I++
			g := emGwt{sourceFrame: r.Expect(tFrameID)}
			r.Expect(tGiven)
			g.givenStatements = statements()
			if r.Accept(tWhen) {
				g.whenStatements = statements()
			}
			r.Expect(tThen)
			g.thenStatements = statements()
			a.gwtEntities = append(a.gwtEntities, g)
		default:
			r.Fail("a frame, an entity, a data block, a note, or a specification")
		}
	}
	return a
}
