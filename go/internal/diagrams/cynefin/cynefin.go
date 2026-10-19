// Package cynefin draws Mermaid's Cynefin diagram.
package cynefin

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the Cynefin diagram type.
var Diagram = &core.Diagram{
	Type: "cynefin",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildCynefin(parseCynefin(source), title)
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderCynefin(model.(*Model), config, options)
	},
}
