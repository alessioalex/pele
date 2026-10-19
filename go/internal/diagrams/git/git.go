// Package git draws Mermaid's gitGraph diagram.
package git

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the gitGraph diagram type.
var Diagram = &core.Diagram{
	Type: "gitGraph",
	Parse: func(source string, config *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildGit(parseGit(source), config, title)
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderGit(model.(*Model), config, options)
	},
}
