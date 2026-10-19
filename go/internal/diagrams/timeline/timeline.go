// Package timeline draws Mermaid's timeline diagram.
package timeline

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the timeline diagram type.
var Diagram = &core.Diagram{
	Type: "timeline",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		db := newDb()
		db.Title = title
		parseTimeline(source, db)
		return db
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderTimeline(model.(*Db), config, options)
	},
}
