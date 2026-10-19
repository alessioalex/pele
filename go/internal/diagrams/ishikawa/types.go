package ishikawa

// Node is a line of the outline: the effect, a category of causes, or a cause.
type Node struct {
	Text     string
	Children []*Node
}
