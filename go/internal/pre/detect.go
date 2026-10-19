package pre

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// Detected is a diagram type and the name Mermaid reports for it, such as `flowchart-v2` for
// `graph TD`.
type Detected struct {
	Type        string
	MermaidType string
}

type detector struct {
	typ, mermaid string
	test         func(s string) bool
}

func skipLeadingSpace(s string) string { return js.TrimStart(s) }

func wordAt(s string, i int) bool {
	r, _ := js.RuneAt(s, i)
	return i < len(s) && js.IsWord(r)
}

// starts is /^\s*prefix/.
func starts(prefix string) func(string) bool {
	return func(s string) bool { return strings.HasPrefix(skipLeadingSpace(s), prefix) }
}

// startsFold is /^\s*prefix/i, for an ASCII prefix.
func startsFold(prefix string) func(string) bool {
	return func(s string) bool { return js.HasPrefixFold(skipLeadingSpace(s), prefix) }
}

// bounded is /^\s*prefix\b/, with the prefix ending in a word character.
func bounded(prefix string, fold bool) func(string) bool {
	return func(s string) bool {
		t := skipLeadingSpace(s)
		if fold {
			if !js.HasPrefixFold(t, prefix) {
				return false
			}
		} else if !strings.HasPrefix(t, prefix) {
			return false
		}
		return !wordAt(t, len(prefix))
	}
}

// Mermaid's detectors, in the order Mermaid registers them. The first match wins.
var detectors = []detector{
	{"flowchart", "flowchart-elk", starts("flowchart-elk")},
	{"mindmap", "mindmap", starts("mindmap")},
	{"architecture", "architecture", starts("architecture")},
	{"agentflow", "agentflow", bounded("agentflow-beta", false)},
	// Mermaid's pattern anchors only its first alternative, so the others match anywhere.
	{"c4", "c4", func(s string) bool {
		return starts("C4Context")(s) || strings.Contains(s, "C4Container") || strings.Contains(s, "C4Component") ||
			strings.Contains(s, "C4Dynamic") || strings.Contains(s, "C4Deployment")
	}},
	{"kanban", "kanban", starts("kanban")},
	{"class", "classDiagram", starts("classDiagram")},
	{"er", "er", starts("erDiagram")},
	{"gantt", "gantt", starts("gantt")},
	{"info", "info", starts("info")},
	{"pie", "pie", starts("pie")},
	{"requirement", "requirement", starts("requirement")},
	{"sequence", "sequence", starts("sequenceDiagram")},
	{"swimlane", "swimlane", bounded("swimlane-beta", false)},
	{"flowchart", "flowchart-v2", func(s string) bool { return starts("graph")(s) || starts("flowchart")(s) }},
	{"timeline", "timeline", starts("timeline")},
	{"gitGraph", "gitGraph", starts("gitGraph")},
	{"state", "stateDiagram", starts("stateDiagram")},
	{"journey", "journey", starts("journey")},
	{"quadrantChart", "quadrantChart", starts("quadrantChart")},
	{"sankey", "sankey", starts("sankey")},
	{"packet", "packet", starts("packet")},
	{"xychart", "xychart", starts("xychart")},
	{"block", "block", starts("block")},
	{"eventmodeling", "eventmodeling", starts("eventmodeling")},
	{"treeView", "treeView", starts("treeView-beta")},
	{"radar", "radar", starts("radar-beta")},
	// `ishikawa(-beta)?\b` comes down to a boundary after `ishikawa`, as `-` is not a word character.
	{"ishikawa", "ishikawa", bounded("ishikawa", true)},
	{"treemap", "treemap", starts("treemap")},
	{"railroad", "railroad", startsFold("railroad-beta")},
	{"railroad", "railroadEbnf", startsFold("railroad-ebnf-beta")},
	{"railroad", "railroadAbnf", startsFold("railroad-abnf-beta")},
	{"railroad", "railroadPeg", startsFold("railroad-peg-beta")},
	{"venn", "venn", starts("venn-beta")},
	{"wardley", "wardley", startsFold("wardley-beta")},
	{"cynefin", "cynefin", func(s string) bool {
		t := skipLeadingSpace(s)
		if !strings.HasPrefix(t, "cynefin-beta") {
			return false
		}
		r, _ := js.RuneAt(t, len("cynefin-beta"))
		return len(t) == len("cynefin-beta") || js.IsSpace(r) || r == ':'
	}},
	{"usecase", "usecase", func(s string) bool {
		t := skipLeadingSpace(s)
		if !strings.HasPrefix(t, "usecase-beta") {
			return false
		}
		r, _ := js.RuneAt(t, len("usecase-beta"))
		return len(t) == len("usecase-beta") || js.IsSpace(r)
	}},
}

// DetectDiagram finds the diagram type of preprocessed text.
func DetectDiagram(cleaned string) (Detected, bool) {
	for _, d := range detectors {
		if d.test(cleaned) {
			return Detected{Type: d.typ, MermaidType: d.mermaid}, true
		}
	}
	return Detected{}, false
}
