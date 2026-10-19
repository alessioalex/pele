// Package eventmodeling draws Mermaid's event modeling diagram.
package eventmodeling

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the event modeling diagram type.
var Diagram = &core.Diagram{
	Type:         "eventmodeling",
	KeepComments: true,
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildEventModel(parseEventModel(source), title)
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderEventModel(model.(*Model), config, options)
	},
}
