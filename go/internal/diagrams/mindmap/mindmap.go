// Package mindmap draws Mermaid's mindmap.
package mindmap

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the mindmap diagram type.
var Diagram = &core.Diagram{
	Type: "mindmap",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		db := &Db{Title: title}
		parseMindmap(source, db)
		return db
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderMindmap(model.(*Db), config, options)
	},
}
