package pele

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/cynefin"
	"github.com/alessioalex/pele/go/internal/diagrams/eventmodeling"
	"github.com/alessioalex/pele/go/internal/diagrams/git"
	"github.com/alessioalex/pele/go/internal/diagrams/info"
	"github.com/alessioalex/pele/go/internal/diagrams/ishikawa"
	"github.com/alessioalex/pele/go/internal/diagrams/journey"
	"github.com/alessioalex/pele/go/internal/diagrams/kanban"
	"github.com/alessioalex/pele/go/internal/diagrams/mindmap"
	"github.com/alessioalex/pele/go/internal/diagrams/packet"
	"github.com/alessioalex/pele/go/internal/diagrams/pie"
	"github.com/alessioalex/pele/go/internal/diagrams/quadrant"
	"github.com/alessioalex/pele/go/internal/diagrams/radar"
	"github.com/alessioalex/pele/go/internal/diagrams/railroad"
	"github.com/alessioalex/pele/go/internal/diagrams/sankey"
	"github.com/alessioalex/pele/go/internal/diagrams/timeline"
	"github.com/alessioalex/pele/go/internal/diagrams/treemap"
	"github.com/alessioalex/pele/go/internal/diagrams/treeview"
	"github.com/alessioalex/pele/go/internal/diagrams/venn"
	"github.com/alessioalex/pele/go/internal/diagrams/wardley"
	"github.com/alessioalex/pele/go/internal/diagrams/xychart"
)

// The diagram types ported so far. See TODO.md for the rest.
var all = []*core.Diagram{
	cynefin.Diagram,
	eventmodeling.Diagram,
	git.Diagram,
	info.Diagram,
	ishikawa.Diagram,
	journey.Diagram,
	kanban.Diagram,
	mindmap.Diagram,
	packet.Diagram,
	pie.Diagram,
	quadrant.Diagram,
	radar.Diagram,
	railroad.Diagram,
	sankey.Diagram,
	timeline.Diagram,
	treemap.Diagram,
	treeview.Diagram,
	venn.Diagram,
	wardley.Diagram,
	xychart.Diagram,
}
