package railroad

// NodeType is the kind of a node of the rule model.
type NodeType int

const (
	Terminal NodeType = iota
	Nonterminal
	Special
	Sequence
	Choice
	Optional
	Repetition
)

// Node is a node of the rule model.
type Node struct {
	Type NodeType
	// A terminal's value, a non-terminal's name, or a special's text.
	Text string
	// A sequence's elements, a choice's alternatives, or the element of an optional or a repetition.
	Kids []*Node
	// A repetition's count. Max is +Inf when the count has no upper bound.
	Min, Max float64
}

// Rule is a named definition.
type Rule struct {
	Name       string
	Definition *Node
}

// Notation is which of Mermaid's four grammars the text was written in.
type Notation string

const (
	NotationRailroad Notation = "railroad"
	NotationEbnf     Notation = "ebnf"
	NotationAbnf     Notation = "abnf"
	NotationPeg      Notation = "peg"
)

// Model is a parsed railroad diagram.
type Model struct {
	Notation Notation
	Title    string
	AccTitle string
	AccDescr string
	// Every rule in source order. A name defined twice appears twice.
	Rules []Rule
}
