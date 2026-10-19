package mindmap

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
)

// Node is a mindmap node.
type Node struct {
	ID       int
	NodeID   string
	Level    int
	Descr    string
	Type     int
	Children []*Node
	IsRoot   bool
	// Index of the first-level branch the node belongs to, wrapping after eleven as in Mermaid.
	// It is 0 for the root, which has none.
	Section int
	// "" when the node has none.
	Class string
	Icon  string
}

const sections = 11

// Db is a parsed mindmap.
type Db struct {
	Title string
	// In source order, which is depth-first. The first node is the root.
	Nodes []*Node

	baseLevel int
	// The chain of nodes that can still take children, with strictly increasing levels.
	open []*Node
}

func (db *Db) GetType(start, end string) int { return common.GetType(start, end) }

func (db *Db) AddNode(level int, id, descr string, typ int, _ *string) {
	isRoot := len(db.Nodes) == 0
	if isRoot {
		db.baseLevel = level
	}
	level -= db.baseLevel

	// Mermaid's parent is the last node with a smaller indentation, wherever it is in the tree.
	depth := len(db.open)
	for depth > 0 && db.open[depth-1].Level >= level {
		depth--
	}
	var parent *Node
	if depth > 0 {
		parent = db.open[depth-1]
	}
	if parent == nil && !isRoot {
		err := core.NewError(`There can be only one root. No parent could be found for ("`+descr+`")`, core.CodeSemantic)
		err.Type = "mindmap"
		panic(err)
	}
	node := &Node{ID: len(db.Nodes), NodeID: id, Level: level, Descr: descr, Type: typ, Children: []*Node{}, IsRoot: isRoot}
	if parent != nil {
		if parent.IsRoot {
			node.Section = len(parent.Children) % sections
		} else {
			node.Section = parent.Section
		}
		parent.Children = append(parent.Children, node)
	}
	db.open = append(db.open[:depth], node)
	db.Nodes = append(db.Nodes, node)
}

// DecorateNode ignores a decoration that comes before any node; Mermaid throws a TypeError.
func (db *Db) DecorateNode(decoration common.Decoration) {
	if len(db.Nodes) == 0 {
		return
	}
	node := db.Nodes[len(db.Nodes)-1]
	if decoration.Icon != "" {
		node.Icon = decoration.Icon
	}
	if decoration.Class != "" {
		node.Class = decoration.Class
	}
}
