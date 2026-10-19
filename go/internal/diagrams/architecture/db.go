//go:build wip

package architecture

import (
	"strconv"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Node is a service or a junction. In "" is no parent; Icon, IconText and Title "" are unset.
type Node struct {
	ID       string
	Junction bool
	Icon     string
	IconText string
	Title    string
	In       string
}

// Group is a group of nodes and groups.
type Group struct {
	ID    string
	Icon  string
	Title string
	In    string
}

// Edge joins two nodes, each at a side.
type Edge struct {
	LhsID    string
	LhsDir   Side
	LhsInto  bool
	LhsGroup bool
	RhsID    string
	RhsDir   Side
	RhsInto  bool
	RhsGroup bool
	Title    string
}

// LayoutHint is an align directive: members in a row or a column.
type LayoutHint struct {
	Direction string
	Members   []string
}

// Model is a parsed architecture diagram. Nodes and Groups keep the order they were added in.
type Model struct {
	Title       string
	AccTitle    string
	AccDescr    string
	Nodes       []*Node
	Groups      []*Group
	Edges       []Edge
	LayoutHints []LayoutHint

	nodeByID   map[string]*Node
	registered map[string]string
}

func newModel() *Model {
	return &Model{nodeByID: map[string]*Node{}, registered: map[string]string{}}
}

func fail(message string) {
	panic(&core.Error{Message: message, Code: core.CodeSemantic, Type: "architecture"})
}

func (m *Model) register(kind, id, parent string) {
	if other, ok := m.registered[id]; ok {
		fail("The " + kind + " id [" + id + "] is already in use by another " + other)
	}
	if parent != "" {
		if id == parent {
			fail("The " + kind + " [" + id + "] cannot be placed within itself")
		}
		p, ok := m.registered[parent]
		if !ok {
			fail("The " + kind + " [" + id + "]'s parent does not exist. Please make sure the parent is created before this " + kind)
		}
		if p == "node" {
			fail("The " + kind + " [" + id + "]'s parent is not a group")
		}
	}
	if kind == "group" {
		m.registered[id] = "group"
	} else {
		m.registered[id] = "node"
	}
}

func (m *Model) addService(s serviceAst) {
	m.register("service", s.id, s.in)
	n := &Node{ID: s.id, Icon: s.icon, IconText: s.iconText, Title: s.title, In: s.in}
	m.Nodes = append(m.Nodes, n)
	m.nodeByID[s.id] = n
}

func (m *Model) addJunction(j junctionAst) {
	m.register("junction", j.id, j.in)
	n := &Node{ID: j.id, Junction: true, In: j.in}
	m.Nodes = append(m.Nodes, n)
	m.nodeByID[j.id] = n
}

func (m *Model) addGroup(g groupAst) {
	m.register("group", g.id, g.in)
	m.Groups = append(m.Groups, &Group{ID: g.id, Icon: g.icon, Title: g.title, In: g.in})
}

func (m *Model) addEdge(e edgeAst) {
	type end struct {
		side    string
		id      string
		crosses bool
	}
	ends := [2]end{{"left", e.lhsID, e.lhsGroup}, {"right", e.rhsID, e.rhsGroup}}
	for _, x := range ends {
		if _, ok := m.registered[x.id]; !ok {
			fail("The " + x.side + "-hand id [" + x.id + "] does not yet exist. Please create the service/group before declaring an edge to it.")
		}
	}
	// Mermaid lets a group id through the check above and then fails with a TypeError.
	for _, x := range ends {
		if m.nodeByID[x.id] == nil {
			fail("The " + x.side + "-hand id [" + x.id + "] is a group. Add {group} to a service inside it instead.")
		}
	}
	parent := m.nodeByID[e.lhsID].In
	if parent != "" && parent == m.nodeByID[e.rhsID].In {
		for _, x := range ends {
			if x.crosses {
				fail("The " + x.side + "-hand id [" + x.id + "] is modified to traverse the group boundary, but the edge does not pass through two groups.")
			}
		}
	}
	m.Edges = append(m.Edges, Edge{
		LhsID: e.lhsID, LhsDir: e.lhsDir, LhsInto: e.lhsInto, LhsGroup: e.lhsGroup,
		RhsID: e.rhsID, RhsDir: e.rhsDir, RhsInto: e.rhsInto, RhsGroup: e.rhsGroup,
		Title: e.title,
	})
}

func (m *Model) addLayoutHint(hint LayoutHint) {
	if len(hint.Members) < 2 {
		fail("An align directive requires at least two members; got " + strconv.Itoa(len(hint.Members)))
	}
	seen := map[string]bool{}
	for _, id := range hint.Members {
		if m.registered[id] != "node" {
			fail("align " + hint.Direction + " references [" + id + "], which is not a service or junction")
		}
		if seen[id] {
			fail("align " + hint.Direction + " lists [" + id + "] more than once")
		}
		seen[id] = true
	}
	m.LayoutHints = append(m.LayoutHints, hint)
}

// collapseIndents is .replace(/\n\s+/g, '\n').
func collapseIndents(s string) string {
	b := make([]byte, 0, len(s))
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
			b = append(b, '\n')
			i = j
			continue
		}
		b = append(b, s[i])
		i++
	}
	return string(b)
}

// populate is Mermaid's populateDb: every group, then services, junctions, edges, and alignments.
func populate(ast *architectureAst, m *Model) {
	if ast.accDescr != "" {
		m.AccDescr = collapseIndents(ast.accDescr)
	}
	if ast.accTitle != "" {
		m.AccTitle = js.TrimStart(ast.accTitle)
	}
	if ast.title != "" {
		m.Title = ast.title
	}
	for _, g := range ast.groups {
		m.addGroup(g)
	}
	for _, s := range ast.services {
		m.addService(s)
	}
	for _, j := range ast.junctions {
		m.addJunction(j)
	}
	for _, e := range ast.edges {
		m.addEdge(e)
	}
	for _, a := range ast.alignments {
		m.addLayoutHint(LayoutHint{Direction: a.direction, Members: append([]string(nil), a.members...)})
	}
}
