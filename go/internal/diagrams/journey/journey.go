// Package journey draws Mermaid's user journey diagram.
package journey

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the journey diagram type.
var Diagram = &core.Diagram{
	Type: "journey",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		m := &model{title: title}
		parseJourney(source, m)
		return m
	},
	Render: func(m any, _ *core.Object, options *core.Options) core.Rendered {
		return render(m.(*model), options)
	},
}
