// Package pie draws Mermaid's pie chart.
package pie

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the pie diagram type.
var Diagram = &core.Diagram{
	Type: "pie",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildPie(parsePie(source), title)
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderPie(model.(*Model), config, options)
	},
}
