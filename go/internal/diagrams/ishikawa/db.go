package ishikawa

type entry struct {
	level int
	node  *Node
}

// Db is a parsed Ishikawa diagram. The root is the effect; its children are the categories of
// causes, theirs the causes, and so on.
type Db struct {
	Title string
	Root  *Node
	// The line of ancestors of the last node, each with its level.
	stack     []entry
	baseLevel int
	hasBase   bool
}

// addNode adds a line. The first line is the effect. Every later line is a cause, nested by its
// indentation relative to the first cause, so the effect itself may be indented more than the causes.
func (db *Db) addNode(rawLevel int, text string) {
	node := &Node{Text: text, Children: []*Node{}}
	if db.Root == nil {
		db.Root = node
		db.stack = []entry{{0, node}}
		return
	}
	if !db.hasBase {
		db.baseLevel = rawLevel
		db.hasBase = true
	}
	level := max(rawLevel-db.baseLevel+1, 1)
	for len(db.stack) > 1 && db.stack[len(db.stack)-1].level >= level {
		db.stack = db.stack[:len(db.stack)-1]
	}
	parent := db.stack[len(db.stack)-1].node
	parent.Children = append(parent.Children, node)
	db.stack = append(db.stack, entry{level, node})
}
