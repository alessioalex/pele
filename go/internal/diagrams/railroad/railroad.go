// Package railroad draws Mermaid's railroad diagrams, written in Mermaid's own notation, EBNF,
// ABNF or PEG.
package railroad

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the railroad diagram type.
var Diagram = &core.Diagram{
	Type: "railroad",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return parseRailroad(source, title)
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderRailroad(model.(*Model), config, options)
	},
}
