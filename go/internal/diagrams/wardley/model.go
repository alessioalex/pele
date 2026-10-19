package wardley

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Coordinates are percentages: x is evolution, y is visibility.
type node struct {
	id, label string
	x, y      float64
	// "anchor", "component", or "pipeline-component"
	className                  string
	labelOffsetX, labelOffsetY *float64
	inPipeline                 bool
	isPipelineParent           bool
	inertia                    bool
	// "" for none.
	sourceStrategy string
}

type link struct {
	source, target string
	dashed         bool
	label          string
	flow           string
}

type trend struct {
	nodeID           string
	targetX, targetY float64
}

type pipeline struct {
	nodeID       string
	componentIDs []string
}

type annotation struct {
	number float64
	x, y   float64
	text   string
}

type note struct {
	text string
	x, y float64
}

type force struct {
	name string
	x, y float64
}

type model struct {
	title, accTitle, accDescr string
	nodes                     []*node
	links                     []link
	trends                    []trend
	pipelines                 []*pipeline
	annotations               []annotation
	notes                     []note
	accelerators              []force
	deaccelerators            []force
	annotationsBox            *[2]float64
	stages                    []string
	boundaries                []float64
	hasBoundaries             bool
	width, height             float64
	hasSize                   bool
}

func semantic(message string) {
	e := core.NewError(message, core.CodeSemantic)
	e.Type = "wardley"
	panic(e)
}

// toPercent: values up to 1 are fractions; larger ones are already percentages.
func toPercent(value float64, context string) float64 {
	normalized := value
	if value <= 1 {
		normalized = value * 100
	}
	if normalized < 0 || normalized > 100 {
		semantic(context + " must be between 0-1 (decimal) or 0-100 (percentage). Received: " + js.NumberToString(value))
	}
	return normalized
}

func toCoordinates(visibility, evolution float64, context string) (float64, float64) {
	x := toPercent(evolution, context+" evolution")
	return x, toPercent(visibility, context+" visibility")
}

func flowFromPort(port string) string {
	switch port {
	case "+<>":
		return "bidirectional"
	case "+<":
		return "backward"
	case "+>":
		return "forward"
	}
	return ""
}

func flowFromArrow(arrow string) (flow, label string, hasLabel bool) {
	if !strings.HasPrefix(arrow, "+") {
		return "", "", false
	}
	if len(arrow) > 1 && arrow[1] == '\'' && len(arrow) > 2 {
		if end := strings.IndexByte(arrow[2:], '\''); end != -1 {
			label, hasLabel = arrow[2:2+end], true
		}
	}
	switch {
	case strings.Contains(arrow, "<>"):
		flow = "bidirectional"
	case strings.Contains(arrow, "<"):
		flow = "backward"
	case strings.Contains(arrow, ">"):
		flow = "forward"
	}
	return
}

func offsets(l *labelAst) (*float64, *float64) {
	if l == nil {
		return nil, nil
	}
	x, y := l.offsetX, l.offsetY
	if l.negX {
		x = -x
	}
	if l.negY {
		y = -y
	}
	return &x, &y
}

type builder struct {
	order     []string
	nodes     map[string]*node
	pipelines map[string]*pipeline
	labels    map[string]string
	m         *model
}

func (b *builder) addNode(n node, setInertia bool) {
	existing, ok := b.nodes[n.id]
	if !ok {
		b.order = append(b.order, n.id)
		b.nodes[n.id] = &n
		return
	}
	// A node added again keeps its class and label offsets unless the new one sets them.
	if n.labelOffsetX == nil {
		n.labelOffsetX = existing.labelOffsetX
	}
	if n.labelOffsetY == nil {
		n.labelOffsetY = existing.labelOffsetY
	}
	n.inPipeline, n.isPipelineParent = existing.inPipeline, existing.isPipelineParent
	if !setInertia {
		n.inertia, n.sourceStrategy = existing.inertia, existing.sourceStrategy
	}
	*existing = n
}

func (b *builder) resolve(name string) string {
	if _, ok := b.nodes[name]; ok {
		return name
	}
	if b.labels == nil {
		b.labels = map[string]string{}
		for _, id := range b.order {
			if _, ok := b.labels[b.nodes[id].label]; !ok {
				b.labels[b.nodes[id].label] = id
			}
		}
	}
	if id, ok := b.labels[name]; ok {
		return id
	}
	return name
}

// populate is Mermaid's populateDb: it fills the model from the syntax tree, in Mermaid's order.
func populate(a *wardleyAst) *model {
	m := &model{}
	b := &builder{nodes: map[string]*node{}, pipelines: map[string]*pipeline{}, m: m}
	if a.size != nil {
		m.width, m.height, m.hasSize = a.size.width, a.size.height, true
	}
	if a.evolution != nil {
		m.hasBoundaries = true
		m.boundaries = []float64{}
		for _, s := range a.evolution {
			name := js.Trim(s.name)
			if s.secondName != "" {
				name += " / " + js.Trim(s.secondName)
			}
			m.stages = append(m.stages, name)
			if s.hasBoundary {
				m.boundaries = append(m.boundaries, s.boundary)
			}
		}
	}
	for _, an := range a.anchors {
		x, y := toCoordinates(an.visibility, an.evolution, `Anchor "`+an.name+`"`)
		b.addNode(node{id: an.name, label: an.name, x: x, y: y, className: "anchor"}, false)
	}
	for _, c := range a.components {
		x, y := toCoordinates(c.visibility, c.evolution, `Component "`+c.name+`"`)
		ox, oy := offsets(c.label)
		b.addNode(node{id: c.name, label: c.name, x: x, y: y, className: "component", labelOffsetX: ox, labelOffsetY: oy,
			inertia: c.inertia, sourceStrategy: c.strategy}, true)
	}
	for _, n := range a.notes {
		x, y := toCoordinates(n.visibility, n.evolution, `Note "`+n.text+`"`)
		m.notes = append(m.notes, note{n.text, x, y})
	}
	var pipelineOrder []string
	for _, p := range a.pipelines {
		parent, ok := b.nodes[p.parent]
		if !ok {
			semantic(`Pipeline "` + p.parent + `" must reference an existing component with coordinates.`)
		}
		y := parent.y
		if _, ok := b.pipelines[p.parent]; !ok {
			pipelineOrder = append(pipelineOrder, p.parent)
		}
		pl := &pipeline{nodeID: p.parent, componentIDs: []string{}}
		b.pipelines[p.parent] = pl
		parent.isPipelineParent = true
		for _, c := range p.components {
			id := p.parent + "_" + c.name
			ox, oy := offsets(c.label)
			x := toPercent(c.evolution, `Pipeline component "`+c.name+`" evolution`)
			b.addNode(node{id: id, label: c.name, x: x, y: y, className: "pipeline-component", labelOffsetX: ox, labelOffsetY: oy}, true)
			pl.componentIDs = append(pl.componentIDs, id)
			b.nodes[id].inPipeline = true
		}
	}
	for _, l := range a.links {
		dashed := l.arrow != "" && (strings.Contains(l.arrow, "-.->") || strings.Contains(l.arrow, ".-."))
		arrowFlow, label, hasLabel := flowFromArrow(l.arrow)
		flow := flowFromPort(l.fromPort)
		if flow == "" {
			flow = flowFromPort(l.toPort)
		}
		if flow == "" {
			flow = arrowFlow
		}
		if !hasLabel {
			label = l.linkLabel
		}
		m.links = append(m.links, link{source: b.resolve(l.from), target: b.resolve(l.to), dashed: dashed, label: label, flow: flow})
	}
	trendAt := map[string]int{}
	for _, ev := range a.evolves {
		n, ok := b.nodes[ev.component]
		if !ok {
			continue
		}
		t := trend{ev.component, toPercent(ev.target, `Evolve target for "`+ev.component+`"`), n.y}
		if i, ok := trendAt[t.nodeID]; ok {
			m.trends[i] = t
		} else {
			trendAt[t.nodeID] = len(m.trends)
			m.trends = append(m.trends, t)
		}
	}
	if len(a.annotations) > 0 {
		x, y := toCoordinates(a.annotations[0][0], a.annotations[0][1], "Annotations box")
		m.annotationsBox = &[2]float64{x, y}
	}
	for _, an := range a.annotation {
		x, y := toCoordinates(an.x, an.y, "Annotation "+js.NumberToString(an.number))
		m.annotations = append(m.annotations, annotation{an.number, x, y, an.text})
	}
	for _, f := range a.accelerators {
		x, y := toCoordinates(f.x, f.y, `Accelerator "`+f.name+`"`)
		m.accelerators = append(m.accelerators, force{f.name, x, y})
	}
	for _, f := range a.deaccelerators {
		x, y := toCoordinates(f.x, f.y, `Deaccelerator "`+f.name+`"`)
		m.deaccelerators = append(m.deaccelerators, force{f.name, x, y})
	}
	for _, id := range b.order {
		m.nodes = append(m.nodes, b.nodes[id])
	}
	for _, id := range pipelineOrder {
		m.pipelines = append(m.pipelines, b.pipelines[id])
	}
	return m
}

// collapseIndents is .replace(/\n\s+/g, '\n').
func collapseIndents(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			j := i + 1
			for j < len(s) {
				r, size := js.RuneAt(s, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			out.WriteByte('\n')
			i = j
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func buildWardley(a *wardleyAst, title string) *model {
	m := populate(a)
	m.title = a.title
	if m.title == "" {
		m.title = title
	}
	if a.accTitle != "" {
		m.accTitle = js.TrimStart(a.accTitle)
	}
	if a.accDescr != "" {
		m.accDescr = collapseIndents(a.accDescr)
	}
	return m
}
