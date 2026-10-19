// Package xychart draws Mermaid's xy chart: bars and lines over a category or a numeric axis.
package xychart

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the xychart diagram type.
var Diagram = &core.Diagram{
	Type:    "xychart",
	Section: "xyChart",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		db := newDb()
		db.Title = title
		parseXyChart(source, db)
		return db
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return render(model.(*Db), config, options)
	},
}
