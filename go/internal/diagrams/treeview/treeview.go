// Package treeview draws Mermaid's treeView diagrams.
package treeview

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the treeView diagram type.
var Diagram = &core.Diagram{
	Type: "treeView",
	Parse: func(source string, _ *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildTreeView(readTreeView(source), title)
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderTreeView(model.(*Model), config, options)
	},
}
