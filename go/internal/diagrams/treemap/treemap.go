// Package treemap draws Mermaid's treemap diagram.
package treemap

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the treemap diagram type.
var Diagram = &core.Diagram{
	Type: "treemap",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildTreemap(parseTreemap(source), title)
	},
	Render: func(m any, config *core.Object, options *core.Options) core.Rendered {
		return render(m.(*model), config, options)
	},
}
