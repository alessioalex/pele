// Package quadrant draws Mermaid's quadrant chart.
package quadrant

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the quadrantChart diagram type.
var Diagram = &core.Diagram{
	Type: "quadrantChart",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		db := newDb()
		db.Title = title
		parseQuadrant(source, db)
		return db
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return render(model.(*Db), config, options)
	},
}
