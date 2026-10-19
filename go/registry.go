package pele

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/info"
)

// Every implemented diagram type.
var all = []*core.Diagram{
	info.Diagram,
}
