package treeview

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// TreeNode is an entry of the tree. Icon, CSSClass and Description are "" when not given.
type TreeNode struct {
	ID        int
	Level     int
	Name      string
	Directory bool
	// An icon name as written (`pack:name`, `file`, `folder`), or `none` to hide the icon.
	Icon        string
	CSSClass    string
	Description string
	Children    []*TreeNode
}

// Model is a parsed treeView diagram.
type Model struct {
	Title    string
	AccTitle string
	AccDescr string
	// A directory named `/` at level -1 that holds the top-level entries.
	Root  *TreeNode
	Count int
}

// treeViewDB is Mermaid's treeView database: nodes arrive in source order with their indentation.
type treeViewDB struct {
	count int
	root  *TreeNode
	stack []*TreeNode
}

func newDB() *treeViewDB {
	root := &TreeNode{ID: 0, Level: -1, Name: "/", Directory: true}
	return &treeViewDB{count: 1, root: root, stack: []*TreeNode{root}}
}

func (db *treeViewDB) addNode(level int, name string, directory bool, cssClass, icon, description string) {
	for level <= db.stack[len(db.stack)-1].Level {
		db.stack = db.stack[:len(db.stack)-1]
	}
	node := &TreeNode{ID: db.count, Level: level, Name: name, Directory: directory, Icon: icon, CSSClass: cssClass, Description: description}
	db.count++
	parent := db.stack[len(db.stack)-1]
	parent.Children = append(parent.Children, node)
	db.stack = append(db.stack, node)
}

func orEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func populate(ast treeAST, db *treeViewDB) {
	for _, node := range ast.nodes {
		level := 0
		if node.indent >= 0 {
			level = node.indent
		}
		name := node.name
		directory := strings.HasSuffix(name, "/")
		if directory {
			name = name[:len(name)-1]
		}
		icon := ""
		if node.iconAnnotation != nil {
			icon = *node.iconAnnotation
			if icon == "" {
				icon = "none"
			}
		}
		db.addNode(level, name, directory, orEmpty(node.classAnnotation), icon, orEmpty(node.descAnnotation))
	}
}

// readTreeView reads the source to a syntax tree, accepting box-drawing input. Errors name lines
// of the original text.
func readTreeView(source string) treeAST {
	text, m := preprocessBoxDrawing(source)
	if len(m) == 0 {
		return parseTreeView(text)
	}
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		err, ok := r.(*core.Error)
		if !ok {
			panic(r)
		}
		message := err.Message
		if head := strings.IndexByte(message, '\n'); head == -1 {
			message = remapErrorLines(message, m)
		} else {
			message = remapErrorLines(message[:head], m) + message[head:]
		}
		line := err.Line
		if original, ok := m[err.Line]; ok {
			line = original
		}
		panic(&core.Error{Message: message, Code: err.Code, Type: err.Type, Line: line, Column: err.Column, Snippet: err.Snippet})
	}()
	return parseTreeView(text)
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

func buildTreeView(ast treeAST, title string) *Model {
	db := newDB()
	populate(ast, db)
	m := &Model{Title: ast.title, Root: db.root, Count: db.count}
	if m.Title == "" {
		m.Title = title
	}
	if ast.accTitle != "" {
		m.AccTitle = js.TrimStart(ast.accTitle)
	}
	if ast.accDescr != "" {
		m.AccDescr = collapseIndents(ast.accDescr)
	}
	return m
}
