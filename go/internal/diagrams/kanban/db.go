package kanban

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

// Node is a card, or a column with its cards in Items. Empty strings are fields left out.
type Node struct {
	ID         string
	Level      int
	Label      string
	Icon       string
	Assigned   string
	Ticket     string
	Priority   string
	CSSClasses string
	Items      []*Node
}

// Model is a parsed kanban diagram.
type Model struct {
	Title    string
	Sections []*Node
}

// Db receives the outline grammar's actions.
type Db struct {
	Model
	last         *Node
	sectionLevel int
	misplaced    *Node
}

func (db *Db) GetType(start, end string) int { return common.GetType(start, end) }

func (db *Db) AddNode(level int, id, descr string, _ int, shapeData *string) {
	node := &Node{ID: id, Level: level, Label: descr}

	if shapeData != nil {
		var yaml string
		if strings.Contains(*shapeData, "\n") {
			yaml = *shapeData + "\n"
		} else {
			yaml = "{\n" + *shapeData + "\n}"
		}
		doc, err := core.ParseYaml(yaml)
		if err != nil {
			panic(core.NewError(`Metadata of "`+node.ID+`" is not valid YAML. `+err.Error(), core.CodeSyntax))
		}
		// Mermaid throws a TypeError on metadata that is empty or not a mapping. Here it is ignored.
		if obj, ok := doc.(*core.Object); ok {
			shape := ""
			if v := obj.Value("shape"); core.Truthy(v) {
				shape = core.ToString(v)
			}
			if shape != js.ToLower(shape) || strings.Contains(shape, "_") {
				panic(core.NewError("No such shape: "+shape+". Shape names should be lowercase.", core.CodeSemantic))
			}
			set := func(key string, field *string) {
				if v := obj.Value(key); core.Truthy(v) {
					*field = core.ToString(v)
				}
			}
			set("label", &node.Label)
			set("icon", &node.Icon)
			set("assigned", &node.Assigned)
			set("ticket", &node.Ticket)
			set("priority", &node.Priority)
		}
	}

	// A node indented less than the first column becomes a card, and Mermaid rejects whatever follows it.
	if db.misplaced != nil {
		panic(core.NewError(`Items without section detected, found section ("`+db.misplaced.Label+`")`, core.CodeSemantic))
	}
	if len(db.Sections) == 0 {
		db.sectionLevel = level
	}
	db.last = node
	if level == db.sectionLevel {
		node.Items = []*Node{}
		db.Sections = append(db.Sections, node)
	} else {
		section := db.Sections[len(db.Sections)-1]
		section.Items = append(section.Items, node)
		if level < db.sectionLevel {
			db.misplaced = node
		}
	}
}

// Mermaid throws a TypeError when a decoration comes before any node. Here it is ignored.
func (db *Db) DecorateNode(decoration common.Decoration) {
	node := db.last
	if node == nil {
		return
	}
	if decoration.Icon != "" {
		node.Icon = decoration.Icon
	}
	if decoration.Class != "" {
		node.CSSClasses = decoration.Class
	}
}
