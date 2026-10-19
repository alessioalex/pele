// Package venn draws Mermaid's venn diagrams.
package venn

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the venn diagram type.
var Diagram = &core.Diagram{
	Type: "venn",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		db := newDB()
		if title != "" {
			db.Title = title
		}
		parseVenn(source, db)
		return db
	},
	Render: func(model any, _ *core.Object, options *core.Options) core.Rendered {
		return renderVenn(model.(*DB), options)
	},
}
