package text

// IconResolver gives the drawing of a named icon, and false when the host has none. An empty
// drawing for a known name is not the same as no icon at all.
type IconResolver func(name string) (string, bool)

// iconMeasurer carries the host's icons for the render in progress along with the measurer. A
// label leaves room for an icon only when the host has one by that name, and layout has to know
// that before anything is drawn. The TypeScript library keeps this in module state.
type iconMeasurer struct {
	Measurer
	resolve IconResolver
	found   map[string]string
}

// WithIcons returns a measurer that also answers for the host's icons during one render.
func WithIcons(m Measurer, icons IconResolver) Measurer {
	if inner, ok := m.(*iconMeasurer); ok {
		m = inner.Measurer
	}
	return &iconMeasurer{Measurer: m, resolve: icons, found: map[string]string{}}
}

// IconMarkup returns the host's drawing of an icon, or "" when it has none.
func IconMarkup(m Measurer, name string) string {
	im, ok := m.(*iconMeasurer)
	if !ok || im.resolve == nil {
		return ""
	}
	markup, ok := im.found[name]
	if !ok {
		markup, _ = im.resolve(name)
		im.found[name] = markup
	}
	return markup
}
