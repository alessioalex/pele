package sankey

// Node is a node of the diagram.
type Node struct {
	ID    string
	Index int
}

// Link is a flow between two nodes.
type Link struct {
	Source *Node
	Target *Node
	Value  float64
}

// Model is a parsed sankey diagram. Mermaid's sankey grammar has no accessible title or
// description, so the model has none.
type Model struct {
	Nodes  []*Node
	Links  []Link
	Title  string
	lookup map[string]*Node
}

func newModel() *Model { return &Model{lookup: map[string]*Node{}} }

func (m *Model) findOrCreateNode(id string) *Node {
	node, ok := m.lookup[id]
	if !ok {
		node = &Node{ID: id, Index: len(m.Nodes)}
		m.lookup[id] = node
		m.Nodes = append(m.Nodes, node)
	}
	return node
}

func (m *Model) addLink(source, target *Node, value float64) {
	m.Links = append(m.Links, Link{source, target, value})
}
