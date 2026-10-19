// Package sankey draws Mermaid's sankey diagram.
package sankey

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the sankey diagram type.
var Diagram = &core.Diagram{
	Type: "sankey",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		db := newModel()
		db.Title = title
		parseSankey(prepareTextForParsing(source), db)
		return db
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderSankey(model.(*Model), config, options)
	},
}
