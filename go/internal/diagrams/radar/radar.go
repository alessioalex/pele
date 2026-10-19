// Package radar draws Mermaid's radar chart.
package radar

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the radar diagram type.
var Diagram = &core.Diagram{
	Type: "radar",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildRadar(parseRadar(source), title)
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderRadar(model.(*Model), config, options)
	},
}
