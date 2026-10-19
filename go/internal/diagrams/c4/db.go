package c4

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// target is what an argument can be applied to: a shape, a boundary or a relation.
type target interface {
	// field is the known field of that name, or nil.
	field(name string) *string
	named(attr *Named)
	setLabel(label string, attr *Named)
}

func (e *Element) field(name string) *string {
	switch name {
	case "label":
		return &e.Label
	case "type":
		return &e.Type
	case "descr":
		return &e.Descr
	case "techn":
		return &e.Techn
	case "sprite":
		return &e.Sprite
	case "tags":
		return &e.Tags
	case "link":
		return &e.Link
	case "bgColor":
		return &e.BgColor
	case "fontColor":
		return &e.FontColor
	case "borderColor":
		return &e.BorderColor
	case "shadowing":
		return &e.Shadowing
	case "shape":
		return &e.Shape
	case "legendText":
		return &e.LegendText
	case "legendSprite":
		return &e.LegendSprite
	}
	return nil
}

// Mermaid assigns a named argument to whatever property it names, structural ones included.
// Here it reaches only the known fields; any other name is dropped, as nothing reads it.
func (e *Element) named(attr *Named) {
	if f := e.field(attr.Key); f != nil {
		*f = attr.Value
	}
}

func (e *Element) setLabel(label string, attr *Named) { e.Label, e.LabelAttr = label, attr }

func (r *Rel) field(name string) *string {
	switch name {
	case "label":
		return &r.Label
	case "descr":
		return &r.Descr
	case "techn":
		return &r.Techn
	case "sprite":
		return &r.Sprite
	case "tags":
		return &r.Tags
	case "link":
		return &r.Link
	case "textColor":
		return &r.TextColor
	case "lineColor":
		return &r.LineColor
	}
	return nil
}

func (r *Rel) named(attr *Named) {
	switch attr.Key {
	case "offsetX":
		r.OffsetX = js.ParseInt(attr.Value, 0)
	case "offsetY":
		r.OffsetY = js.ParseInt(attr.Value, 0)
	default:
		if f := r.field(attr.Key); f != nil {
			*f = attr.Value
		}
	}
}

func (r *Rel) setLabel(label string, attr *Named) { r.Label, r.LabelAttr = label, attr }

func arg(attrs []Attr, i int) (Attr, bool) {
	if i < 0 || i >= len(attrs) {
		return Attr{}, false
	}
	return attrs[i], true
}

// A positional argument fills the field of its slot. A named one fills the field it names, whichever slot it is in.
func put(t target, field string, attrs []Attr, i int) {
	attr, ok := arg(attrs, i)
	if !ok {
		return
	}
	if attr.Named == nil {
		*t.field(field) = attr.Str
	} else {
		t.named(attr.Named)
	}
}

func setLabel(t target, attrs []Attr, i int) {
	attr, ok := arg(attrs, i)
	switch {
	case !ok:
		t.setLabel("", nil)
	case attr.Named != nil:
		t.setLabel("", attr.Named)
		t.named(attr.Named)
	default:
		t.setLabel(attr.Str, nil)
	}
}

// Db is the parsed diagram.
type Db struct {
	// The header keyword: C4Context, C4Container, C4Component, C4Dynamic or C4Deployment.
	C4Type   string
	Title    string
	AccDescr string
	Shapes   []*Element
	// The first boundary is the unnamed one that holds everything at the top level.
	Boundaries    []*Element
	Rels          []*Rel
	ShapeInRow    float64
	BoundaryInRow float64

	shapeIndex map[string]*Element
	// The top-level boundary is not listed, so a boundary a diagram names `global` is its own box.
	// In Mermaid it replaces the top level and nothing is drawn.
	boundaryIndex map[string]*Element
	relIndex      map[string]map[string]*Rel
	current       *Element
	stack         []*Element
}

func newDb() *Db {
	global := &Element{Alias: "global", Label: "global", Type: "global"}
	return &Db{
		Boundaries:    []*Element{global},
		ShapeInRow:    4,
		BoundaryInRow: 2,
		shapeIndex:    map[string]*Element{},
		boundaryIndex: map[string]*Element{},
		relIndex:      map[string]map[string]*Rel{},
		current:       global,
	}
}

func (db *Db) setC4Type(typ string) { db.C4Type = typ }

func (db *Db) setTitle(title string) { db.Title = title }

func (db *Db) setAccDescription(descr string) { db.AccDescr = collapseIndents(descr) }

// collapseIndents is .replace(/\n\s+/g, '\n').
func collapseIndents(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			j := i + 1
			for {
				ok, size := spaceAt(s, j)
				if !ok {
					break
				}
				j += size
			}
			if j == i+1 {
				b.WriteByte('\n')
				i++
				continue
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

func (db *Db) element(list *[]*Element, index map[string]*Element, attrs []Attr) *Element {
	alias := attrs[0]
	var el *Element
	if alias.Named == nil {
		el = index[alias.Str]
	}
	if el == nil {
		el = &Element{}
		*list = append(*list, el)
		if alias.Named == nil {
			el.Alias = alias.Str
			index[alias.Str] = el
		} else {
			el.AliasAttr = alias.Named
			el.named(alias.Named)
		}
	}
	setLabel(el, attrs, 1)
	return el
}

// alias, label, ?descr, ?sprite, ?tags, $link
func (db *Db) addPersonOrSystem(kind string, attrs []Attr) {
	shape := db.element(&db.Shapes, db.shapeIndex, attrs)
	put(shape, "descr", attrs, 2)
	db.finishShape(shape, kind, attrs, 3)
}

// alias, label, ?techn, ?descr, ?sprite, ?tags, $link
func (db *Db) addContainer(kind string, attrs []Attr) {
	shape := db.element(&db.Shapes, db.shapeIndex, attrs)
	put(shape, "techn", attrs, 2)
	put(shape, "descr", attrs, 3)
	db.finishShape(shape, kind, attrs, 4)
}

func (db *Db) addComponent(kind string, attrs []Attr) { db.addContainer(kind, attrs) }

func (db *Db) finishShape(shape *Element, kind string, attrs []Attr, at int) {
	put(shape, "sprite", attrs, at)
	put(shape, "tags", attrs, at+1)
	put(shape, "link", attrs, at+2)
	shape.Kind = kind
	shape.Parent = db.current
}

func (db *Db) boundary(attrs []Attr, fallback string) *Element {
	b := db.element(&db.Boundaries, db.boundaryIndex, attrs)
	if _, ok := arg(attrs, 2); !ok {
		b.Type = fallback
	} else {
		put(b, "type", attrs, 2)
	}
	return b
}

func (db *Db) open(b *Element) {
	b.Parent = db.current
	db.stack = append(db.stack, db.current)
	db.current = b
}

// alias, label, ?type, ?tags, $link
func (db *Db) addPersonOrSystemBoundary(attrs []Attr, fallback string) {
	b := db.boundary(attrs, fallback)
	put(b, "tags", attrs, 3)
	put(b, "link", attrs, 4)
	db.open(b)
}

func (db *Db) addContainerBoundary(attrs []Attr) { db.addPersonOrSystemBoundary(attrs, "container") }

// alias, label, ?type, ?descr, ?sprite, ?tags, $link
func (db *Db) addDeploymentNode(nodeType string, attrs []Attr) {
	b := db.boundary(attrs, "node")
	put(b, "descr", attrs, 3)
	// Mermaid drops whatever is in the sprite slot, a named argument included.
	put(b, "sprite", attrs, 4)
	put(b, "tags", attrs, 5)
	put(b, "link", attrs, 6)
	b.NodeType = nodeType
	db.open(b)
}

func (db *Db) popBoundaryParseStack() {
	if n := len(db.stack); n > 0 {
		db.current = db.stack[n-1]
		db.stack = db.stack[:n-1]
	} else {
		db.current = db.Boundaries[0]
	}
}

// from, to, label, ?techn, ?descr, ?sprite, ?tags, $link
func (db *Db) addRel(typ string, attrs []Attr) {
	from, ok1 := arg(attrs, 0)
	to, ok2 := arg(attrs, 1)
	if _, ok3 := arg(attrs, 2); !ok1 || !ok2 || !ok3 {
		return
	}
	known := from.Named == nil && to.Named == nil
	var rel *Rel
	if known {
		rel = db.relIndex[from.Str][to.Str]
	}
	if rel == nil {
		rel = &Rel{Type: typ}
		db.Rels = append(db.Rels, rel)
		if known {
			targets := db.relIndex[from.Str]
			if targets == nil {
				targets = map[string]*Rel{}
				db.relIndex[from.Str] = targets
			}
			targets[to.Str] = rel
		}
		if from.Named == nil {
			rel.From = from.Str
		} else {
			rel.FromAttr = from.Named
		}
		if to.Named == nil {
			rel.To = to.Str
		} else {
			rel.ToAttr = to.Named
		}
	}
	rel.Type = typ
	setLabel(rel, attrs, 2)
	put(rel, "techn", attrs, 3)
	put(rel, "descr", attrs, 4)
	put(rel, "sprite", attrs, 5)
	put(rel, "tags", attrs, 6)
	put(rel, "link", attrs, 7)
}

func (db *Db) getC4Shape(alias string) *Element {
	if el := db.shapeIndex[alias]; el != nil {
		return el
	}
	return db.boundaryIndex[alias]
}

// elementName, ?bgColor, ?fontColor, ?borderColor, ?shadowing, ?shape, ?sprite, ?techn, ?legendText, ?legendSprite
func (db *Db) updateElStyle(attrs []Attr) {
	if attrs[0].Named != nil {
		return
	}
	el := db.getC4Shape(attrs[0].Str)
	if el == nil {
		return
	}
	put(el, "bgColor", attrs, 1)
	put(el, "fontColor", attrs, 2)
	put(el, "borderColor", attrs, 3)
	put(el, "shadowing", attrs, 4)
	put(el, "shape", attrs, 5)
	put(el, "sprite", attrs, 6)
	put(el, "techn", attrs, 7)
	put(el, "legendText", attrs, 8)
	put(el, "legendSprite", attrs, 9)
}

// from, to, ?textColor, ?lineColor, ?offsetX, ?offsetY
func (db *Db) updateRelStyle(attrs []Attr) {
	from, ok1 := arg(attrs, 0)
	to, ok2 := arg(attrs, 1)
	if !ok1 || !ok2 || from.Named != nil || to.Named != nil {
		return
	}
	rel := db.relIndex[from.Str][to.Str]
	if rel == nil {
		return
	}
	put(rel, "textColor", attrs, 2)
	put(rel, "lineColor", attrs, 3)
	if x, ok := arg(attrs, 4); ok {
		if x.Named == nil {
			rel.OffsetX = js.ParseInt(x.Str, 0)
		} else {
			rel.named(x.Named)
		}
	}
	if y, ok := arg(attrs, 5); ok {
		if y.Named == nil {
			rel.OffsetY = js.ParseInt(y.Str, 0)
		} else {
			rel.named(y.Named)
		}
	}
}

// ?c4ShapeInRow, ?c4BoundaryInRow. As in Mermaid, the slot decides which one a value sets, not its name.
func (db *Db) updateLayoutConfig(attrs []Attr) {
	count := func(i int) float64 {
		attr, ok := arg(attrs, i)
		switch {
		case !ok:
			return js.ParseInt("", 0)
		case attr.Named != nil:
			return js.ParseInt(attr.Named.Value, 0)
		}
		return js.ParseInt(attr.Str, 0)
	}
	shapes := count(0)
	boundaries := count(1)
	if shapes >= 1 {
		db.ShapeInRow = shapes
	}
	if boundaries >= 1 {
		db.BoundaryInRow = boundaries
	}
}
