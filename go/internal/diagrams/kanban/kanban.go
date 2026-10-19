// Package kanban draws Mermaid's kanban diagram.
package kanban

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the kanban diagram type.
var Diagram = &core.Diagram{
	Type: "kanban",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		db := &Db{}
		db.Title = title
		parseKanban(source, db)
		return &db.Model
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderKanban(model.(*Model), config, options)
	},
}
