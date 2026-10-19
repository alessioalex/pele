package c4

// Named is a `$key="value"` argument.
type Named struct {
	Key   string
	Value string
}

// Attr is a positional argument, or a named one when Named is set.
type Attr struct {
	Str   string
	Named *Named
}

// Element is a shape or a boundary. Fields the TypeScript leaves undefined are "" here: everything
// that reads them only tests whether they are set.
type Element struct {
	Alias string
	Label string
	// A named argument written where the alias or the label belongs. It is applied to the field it names.
	AliasAttr *Named
	LabelAttr *Named
	Parent    *Element
	Type      string
	Descr     string
	Techn     string
	Sprite    string
	Tags      string
	Link      string
	BgColor   string
	FontColor string
	// Shapes: Mermaid's typeC4Shape (person, external_system_db, container_queue and so on).
	Kind         string
	BorderColor  string
	Shadowing    string
	Shape        string
	LegendText   string
	LegendSprite string
	// Set on deployment nodes: node, nodeL or nodeR.
	NodeType string
}

// Rel is a relation.
type Rel struct {
	Type      string
	From      string
	To        string
	Label     string
	FromAttr  *Named
	ToAttr    *Named
	LabelAttr *Named
	Techn     string
	Descr     string
	Sprite    string
	Tags      string
	Link      string
	TextColor string
	LineColor string
	// NaN when the value did not parse; 0 when unset. Both mean no offset.
	OffsetX float64
	OffsetY float64
}
