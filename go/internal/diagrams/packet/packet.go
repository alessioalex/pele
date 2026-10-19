// Package packet draws Mermaid's packet diagram.
package packet

import "github.com/alessioalex/pele/go/internal/core"

// Diagram is the packet diagram type.
var Diagram = &core.Diagram{
	Type: "packet",
	Parse: func(source string, config *core.Object, title string, _ int, _ core.ParseLimits) any {
		return buildPacket(parsePacket(source), config, title)
	},
	Render: func(model any, config *core.Object, options *core.Options) core.Rendered {
		return renderPacket(model.(*Model), config, options)
	},
}
