package git

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

const (
	tCherryPick = iota
	tGitGraphColon
	tHighlight
	tGitGraph
	tCheckout
	tReverse
	tParent
	tCommit
	tNormal
	tBranch
	tOrder
	tSwitch
	tType
	tMerge
	tMsg
	tTag
	tID
	tLR
	tTB
	tBT
	tColon
	tAccDescr
	tAccTitle
	tTitle
	tInt
	tString
	tNewline
	tReference = 31
)

func isNameChar(c byte) bool { return c == '-' || c == '.' || c == '/' || js.IsWord(rune(c)) }

// matchReference is /\w([-\./\w]*[-\w])?/y.
func matchReference(src string, at int) int {
	if at >= len(src) || !js.IsWord(rune(src[at])) {
		return -1
	}
	j := at + 1
	for j < len(src) && isNameChar(src[j]) {
		j++
	}
	for e := j; e > at+1; e-- {
		if c := src[e-1]; c != '.' && c != '/' {
			return e
		}
	}
	return at + 1
}

// matchInt is /0|[1-9][0-9]*(?!\.)/y.
func matchInt(src string, at int) int {
	if at >= len(src) {
		return -1
	}
	if src[at] == '0' {
		return at + 1
	}
	if src[at] < '1' || src[at] > '9' {
		return -1
	}
	j := at + 1
	for j < len(src) && src[j] >= '0' && src[j] <= '9' {
		j++
	}
	if j < len(src) && src[j] == '.' {
		// Backtracking gives up the last digit, which is not a dot.
		if j-1 > at {
			return j - 1
		}
		return -1
	}
	return j
}

const wordChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz"

var reference = &common.TokenType{Name: "REFERENCE", Match: matchReference, First: wordChars}
var name = []*common.TokenType{reference}

// A keyword that is the start of a longer name is part of that name: `commits` is a branch.
func word(w string) *common.TokenType {
	return &common.TokenType{Name: w, Literal: w, Longer: name}
}

func lit(w string) *common.TokenType { return &common.TokenType{Name: w, Literal: w} }

func gitGraphKeyword() *common.TokenType {
	t := *common.Keyword("gitGraph")
	t.Longer = name
	return &t
}

// accDescrOpener is /[\t ]*accDescr\s*{/y.
func accDescrOpener(src string, at int) int {
	p := at
	for p < len(src) && (src[p] == ' ' || src[p] == '\t') {
		p++
	}
	if !strings.HasPrefix(src[p:], "accDescr") {
		return -1
	}
	p += len("accDescr")
	for p < len(src) {
		r, size := js.RuneAt(src, p)
		if !js.IsSpace(r) {
			break
		}
		p += size
	}
	if p < len(src) && src[p] == '{' {
		return p + 1
	}
	return -1
}

func accDescr() *common.TokenType {
	t := *common.ACC_DESCR
	t.Opener = accDescrOpener
	return &t
}

var gitTokens = []*common.TokenType{
	word("cherry-pick"),
	lit("gitGraph:"),
	word("HIGHLIGHT"),
	gitGraphKeyword(),
	word("checkout"),
	word("REVERSE"),
	lit("parent:"),
	word("commit"),
	word("NORMAL"),
	word("branch"),
	lit("order:"),
	word("switch"),
	lit("type:"),
	word("merge"),
	lit("msg:"),
	lit("tag:"),
	lit("id:"),
	word("LR"),
	word("TB"),
	word("BT"),
	lit(":"),
	// Without a closing brace the block form scans to the end of the text, and a brace is not a
	// token on its own, so the first failure is final.
	accDescr(),
	common.ACC_TITLE,
	common.TITLE,
	{Name: "INT", Match: matchInt, First: "0123456789"},
	common.STRING,
	common.NEWLINE,
	common.WHITESPACE,
	common.YAML,
	common.DIRECTIVE,
	common.SINGLE_LINE_COMMENT,
	reference,
}

const (
	stCommit = iota
	stBranch
	stMerge
	stCheckout
	stCherryPicking
)

// statementAst is one of the statements of the TypeScript AST; kind says which, and the fields
// it does not have are left empty.
type statementAst struct {
	kind    int
	id      *string
	message *string
	tags    []string
	typ     string
	name    string
	order   *float64
	branch  string
	parent  *string
}

type gitGraphAst struct {
	dir        string
	title      string
	accTitle   string
	accDescr   string
	statements []*statementAst
}

func strp(s string) *string { return &s }

func parseGit(src string) *gitGraphAst {
	r := common.NewReader(common.Tokenize(src, gitTokens, "gitGraph"), gitTokens, "gitGraph")
	ast := &gitGraphAst{}

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
	branchName := func() string {
		if r.Kind() == tReference {
			return r.Take()
		}
		if r.Kind() == tString {
			return common.StringValue(r.Take())
		}
		r.Fail("a branch name")
		return ""
	}
	str := func() string { return common.StringValue(r.Expect(tString)) }
	commitType := func() string {
		if k := r.Kind(); k != tNormal && k != tReverse && k != tHighlight {
			r.Fail("NORMAL, REVERSE, or HIGHLIGHT")
		}
		return r.Take()
	}

	for r.Kind() == tNewline {
		r.I++
	}
	if !r.Accept(tGitGraphColon) {
		r.Expect(tGitGraph)
		next := r.KindAt(r.I + 1)
		if k := r.Kind(); (k == tLR || k == tTB || k == tBT) && next == tColon {
			ast.dir = r.Take()
			r.I++
		} else {
			r.Accept(tColon)
		}
	}

	for r.Kind() != -1 {
		switch r.Kind() {
		case tNewline:
			r.I++
		case tAccDescr:
			ast.accDescr = common.AccDescrValue(r.Take())
			endOfLine()
		case tAccTitle:
			ast.accTitle = common.AccTitleValue(r.Take())
			endOfLine()
		case tTitle:
			ast.title = common.TitleValue(r.Take())
			endOfLine()
		case tCommit:
			r.I++
			node := &statementAst{kind: stCommit, tags: []string{}}
			for {
				if r.Accept(tID) {
					node.id = strp(str())
				} else if r.Accept(tTag) {
					node.tags = append(node.tags, str())
				} else if r.Accept(tType) {
					node.typ = commitType()
				} else if r.Accept(tMsg) || r.Kind() == tString {
					node.message = strp(str())
				} else {
					break
				}
			}
			endOfLine()
			ast.statements = append(ast.statements, node)
		case tBranch:
			r.I++
			node := &statementAst{kind: stBranch, name: branchName()}
			if r.Accept(tOrder) {
				order := js.ParseInt(r.Expect(tInt), 0)
				node.order = &order
			}
			endOfLine()
			ast.statements = append(ast.statements, node)
		case tMerge:
			r.I++
			node := &statementAst{kind: stMerge, branch: branchName(), tags: []string{}}
			for {
				if r.Accept(tID) {
					node.id = strp(str())
				} else if r.Accept(tTag) {
					node.tags = append(node.tags, str())
				} else if r.Accept(tType) {
					node.typ = commitType()
				} else {
					break
				}
			}
			endOfLine()
			ast.statements = append(ast.statements, node)
		case tCheckout, tSwitch:
			r.I++
			node := &statementAst{kind: stCheckout, branch: branchName()}
			endOfLine()
			ast.statements = append(ast.statements, node)
		case tCherryPick:
			r.I++
			node := &statementAst{kind: stCherryPicking, tags: []string{}}
			for {
				if r.Accept(tID) {
					node.id = strp(str())
				} else if r.Accept(tTag) {
					node.tags = append(node.tags, str())
				} else if r.Accept(tParent) {
					node.parent = strp(str())
				} else {
					break
				}
			}
			endOfLine()
			ast.statements = append(ast.statements, node)
		default:
			r.Fail("a statement, a title, or a line break")
		}
	}
	return ast
}
