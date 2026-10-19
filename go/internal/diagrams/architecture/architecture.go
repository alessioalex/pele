// Package architecture draws Mermaid's architecture diagram.
package architecture

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the architecture diagram type.
var Diagram = &core.Diagram{
	Type: "architecture",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		m := newModel()
		if title != "" {
			m.Title = title
		}
		populate(parseArchitecture(source), m)
		return m
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderArchitecture(model.(*Model), config, options)
	},
}
