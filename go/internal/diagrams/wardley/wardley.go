// Package wardley draws Mermaid's Wardley map.
package wardley

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the wardley diagram type.
var Diagram = &core.Diagram{
	Type:    "wardley",
	Section: "wardley-beta",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildWardley(parseWardley(source), title)
	},
	Render: func(m any, config *core.Object, options *core.Options) core.Rendered {
		return render(m.(*model), config, options)
	},
}
