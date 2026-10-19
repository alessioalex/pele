package venn

// Subset is a set (one id) or an overlap of sets (two or more ids, sorted), with its relative area.
type Subset struct {
	Sets []string
	Size float64
	// Label is nil when none was given.
	Label *string
}

// Text is a line of text shown inside the region of the given sets.
type Text struct {
	Sets  []string
	ID    string
	Label *string
}

// Style targets the sorted ids of a set or overlap, or the id of a text.
type Style struct {
	Targets []string
	Styles  orderedMap
}

// orderedMap is a Map of strings, which iterates in insertion order.
type orderedMap struct {
	keys []string
	vals map[string]string
}

func (m *orderedMap) set(k, v string) {
	if m.vals == nil {
		m.vals = map[string]string{}
	}
	if _, ok := m.vals[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.vals[k] = v
}
