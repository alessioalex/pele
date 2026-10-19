package block

import (
	"math"
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

const maxSpaces = 100000

func typeStr2Type(typeStr string) string {
	switch typeStr {
	case "[]":
		return "square"
	case "()":
		return "round"
	case "(())":
		return "circle"
	case ">]":
		return "rect_left_inv_arrow"
	case "{}":
		return "diamond"
	case "{{}}":
		return "hexagon"
	case "([])":
		return "stadium"
	case "[[]]":
		return "subroutine"
	case "[()]":
		return "cylinder"
	case "((()))":
		return "doublecircle"
	case "[//]":
		return "lean_right"
	case `[\\]`:
		return "lean_left"
	case `[/\]`:
		return "trapezoid"
	case `[\/]`:
		return "inv_trapezoid"
	case "<[]>":
		return "block_arrow"
	default:
		return "na"
	}
}

func edgeStrToEdgeData(typeStr string) string {
	t := js.Trim(typeStr)
	if t == "" {
		return ""
	}
	switch t[len(t)-1] {
	case 'x':
		return "arrow_cross"
	case 'o':
		return "arrow_circle"
	case '>':
		return "arrow_point"
	default:
		return ""
	}
}

func edgeStrToEdgeStartData(typeStr string) string {
	t := js.Trim(typeStr)
	if t == "" {
		return "arrow_open"
	}
	switch t[0] {
	case 'x':
		return "arrow_cross"
	case 'o':
		return "arrow_circle"
	case '<':
		return "arrow_point"
	default:
		return "arrow_open"
	}
}

func edgeStrToThickness(typeStr string) string {
	if strings.Contains(typeStr, "==") {
		return "thick"
	}
	return "normal"
}

func edgeStrToPattern(typeStr string) string {
	if strings.Contains(typeStr, ".-") {
		return "dotted"
	}
	return "solid"
}

type dbFrame struct {
	list     []*Block
	at       int
	parent   *Block
	children []*Block
}

type blockDb struct {
	title   string
	root    *Block
	blocks  []*Block
	edges   []*Block
	classes map[string]*ClassDef
	// Mermaid keeps the root in here too, which makes a block called `root` vanish. Here it stays out.
	byID map[string]*Block
	// Ids that a class or style statement named before any block had them.
	pending   map[string]bool
	edgeCount map[string]int
	ids       int
	colors    int
	spaces    float64
}

func newBlockDb() *blockDb {
	return &blockDb{
		root:      &Block{ID: "root", Type: "composite", Children: []*Block{}, Columns: -1, WidthInColumns: 1},
		classes:   map[string]*ClassDef{},
		byID:      map[string]*Block{},
		pending:   map[string]bool{},
		edgeCount: map[string]int{},
	}
}

// Mermaid's ids are random. These are counted, so the same text always gives the same output.
func (db *blockDb) generateID() string {
	db.ids++
	return "id-" + strconv.Itoa(db.ids)
}

func (db *blockDb) addStyleClass(id, styleAttributes string) {
	found := db.classes[id]
	if found == nil {
		found = &ClassDef{ID: id, Styles: []string{}, TextStyles: []string{}}
		db.classes[id] = found
	}
	for _, attribute := range strings.Split(styleAttributes, ",") {
		fixed := attribute
		if semi := strings.IndexByte(attribute, ';'); semi != -1 {
			fixed = attribute[:semi] + attribute[semi+1:]
		}
		fixed = js.Trim(fixed)
		if strings.Contains(attribute, "color") {
			found.TextStyles = append(found.TextStyles, strings.Replace(strings.Replace(fixed, "fill", "bgFill", 1), "color", "fill", 1))
		}
		found.Styles = append(found.Styles, fixed)
	}
}

func (db *blockDb) addStyle2Node(id, styles string) {
	db.named(id).Styles = strings.Split(styles, ",")
}

// Mermaid looks each id up before trimming it, so `class a, b hot` misses b. Here it is trimmed first.
func (db *blockDb) setCSSClass(itemIDs, cssClassName string) {
	for _, id := range strings.Split(itemIDs, ",") {
		b := db.named(js.Trim(id))
		b.Classes = append(b.Classes, cssClassName)
	}
}

// Mermaid throws on a style for an unknown id, and a class for one makes the block of that id
// disappear when it is declared later. Here both wait for the block.
func (db *blockDb) named(id string) *Block {
	b := db.byID[id]
	if b == nil {
		b = &Block{ID: id, Type: "na", Children: []*Block{}, Columns: -1, WidthInColumns: 1}
		db.byID[id] = b
		db.pending[id] = true
	}
	return b
}

func (db *blockDb) setHierarchy(statements []*Block) {
	stack := []*dbFrame{{list: statements, parent: db.root}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		if f.at == len(f.list) {
			f.parent.Children = f.children
			stack = stack[:len(stack)-1]
			continue
		}
		block := f.list[f.at]
		f.at++
		switch block.Type {
		case "classDef":
			db.addStyleClass(block.ID, block.CSS)
			continue
		case "applyClass":
			db.setCSSClass(block.ID, block.StyleClass)
			continue
		case "applyStyles":
			if block.StylesStr != "" {
				db.addStyle2Node(block.ID, block.StylesStr)
			}
			continue
		case "column-setting":
			f.parent.Columns = block.Columns
			continue
		case "edge":
			count := db.edgeCount[block.ID] + 1
			db.edgeCount[block.ID] = count
			block.ID = strconv.Itoa(count) + "-" + block.ID
			db.edges = append(db.edges, block)
			continue
		}

		// A composite whose header was a link statement has no id of its own.
		if block.NoID {
			block.NoID = false
			block.ID = db.generateID()
		}
		if block.Label == "" {
			if block.Type == "composite" {
				block.Label = ""
			} else {
				block.Label = block.ID
			}
		}
		known := db.byID[block.ID]
		fresh := known == nil
		if !fresh && db.pending[block.ID] {
			delete(db.pending, block.ID)
			fresh = true
		}
		if !fresh {
			if block.Type != "na" {
				known.Type = block.Type
			}
			if block.Label != block.ID {
				known.Label = block.Label
			}
		} else {
			if known != nil {
				block.Classes = known.Classes
				block.Styles = known.Styles
			}
			if block.Type == "composite" {
				block.ColorIndex = db.colors
				db.colors++
			}
			db.byID[block.ID] = block
		}

		inner := block.Children
		block.Children = []*Block{}
		if block.Type == "space" {
			width := block.Width
			if !(width <= maxSpaces-db.spaces) {
				panic(core.NewError("A block diagram can hold at most "+strconv.Itoa(maxSpaces)+" space blocks.", core.CodeLimit))
			}
			db.spaces += math.Max(width, 0)
			for j := 0; float64(j) < width; j++ {
				c := *block
				c.ID = block.ID + "-" + strconv.Itoa(j)
				c.Children = []*Block{}
				db.byID[c.ID] = &c
				f.children = append(f.children, &c)
			}
		} else if fresh {
			f.children = append(f.children, block)
		}
		if len(inner) > 0 {
			stack = append(stack, &dbFrame{list: inner, parent: block})
		}
	}
	db.blocks = db.root.Children
}
