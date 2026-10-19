// Package ishikawa draws Mermaid's Ishikawa (fishbone) diagram.
package ishikawa

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the ishikawa diagram type.
var Diagram = &core.Diagram{
	Type: "ishikawa",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		db := &Db{Title: title}
		parseIshikawa(source, db)
		return db
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderIshikawa(model.(*Db), config, options)
	},
}
