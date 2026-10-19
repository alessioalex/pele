package block

// Block is one shape for blocks, edges, and the statements that set columns, classes, and
// styles, as in Mermaid.
type Block struct {
	ID string
	// NoID is set on a composite whose header was a link statement, until it is given an id.
	NoID     bool
	Type     string
	Label    string
	Children []*Block
	// Columns is -1 when unset.
	Columns float64
	// WidthInColumns is 1 when unset.
	WidthInColumns float64
	// Number of cells a `space` statement asked for.
	Width          float64
	Directions     []string
	Classes        []string
	Styles         []string
	ColorIndex     int
	Start          string
	End            string
	ArrowTypeStart string
	ArrowTypeEnd   string
	Thickness      string
	Pattern        string
	CSS            string
	StyleClass     string
	StylesStr      string
}

// statement is a block, or a statement with links: block, edge, block, and so on.
type statement struct {
	block *Block
	chain []*Block
}

// ClassDef is a classDef statement.
type ClassDef struct {
	ID         string
	Styles     []string
	TextStyles []string
}

// Model is a parsed block diagram.
type Model struct {
	Root    *Block
	Blocks  []*Block
	Edges   []*Block
	Classes map[string]*ClassDef
	ByID    map[string]*Block
	Title   string
}
