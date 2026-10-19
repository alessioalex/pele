// Package engine runs a diagram through preprocessing, detection, parsing and rendering, as
// core.ts does in the TypeScript library.
package engine

import (
	"fmt"
	"runtime/debug"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/pre"
	"github.com/alessioalex/pele/go/internal/text"
)

// Registry maps diagram types to their implementations.
type Registry map[string]*core.Diagram

// NewRegistry registers diagrams by type.
func NewRegistry(list ...*core.Diagram) Registry {
	r := Registry{}
	for _, d := range list {
		r[d.Type] = d
	}
	return r
}

const (
	// Mermaid's default maxTextSize.
	defaultLimit = 50000
	// Mermaid has no limit on its output. A source within the limit can still describe a
	// drawing of many megabytes, which would stall the page it is put into.
	defaultOutputLimit = 4_000_000
	// Mermaid stops at 500. `A & B --> C & D` multiplies: without a limit, a few kilobytes of
	// source ask for millions of edges.
	defaultMaxEdges = 5000
)

// InternalError is a failure of the renderer itself rather than of the diagram text.
type InternalError struct {
	Type  string
	Value any
	Stack string
}

func (e *InternalError) Error() string {
	return fmt.Sprintf("pele: internal error in %s diagram: %v", e.Type, e.Value)
}

// Parsed is diagram text that was read and parsed.
type Parsed struct {
	pre.Detected
	Diagram *core.Diagram
	Model   any
	Config  *core.Object
}

// run calls f, turning a panic with *core.Error into its error, tagged with the diagram type.
func run(found pre.Detected, f func()) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if pe, ok := r.(*core.Error); ok {
			if pe.Type == "" {
				pe.Type = found.Type
			}
			if pe.MermaidType == "" {
				pe.MermaidType = found.MermaidType
			}
			err = pe
			return
		}
		err = &InternalError{Type: found.Type, Value: r, Stack: string(debug.Stack())}
	}()
	f()
	return nil
}

func numberOr(v *float64, def float64) float64 {
	if v != nil {
		return *v
	}
	return def
}

// ParseSource reads and parses diagram text.
func ParseSource(reg Registry, src string, limit, maxEdges *float64, extra *core.Object) (*Parsed, error) {
	lim := numberOr(limit, defaultLimit)
	if float64(js.Len(src)) > lim {
		return nil, core.NewError("Diagram source is longer than the limit of "+js.NumberToString(lim)+" characters.", core.CodeLimit)
	}
	p, err := pre.Preprocess(src)
	if err != nil {
		return nil, err
	}
	found, ok := pre.DetectDiagram(p.Text)
	if !ok {
		return nil, core.NewError("No diagram type detected for the given text.", core.CodeUnsupported)
	}
	diagram := reg[found.Type]
	if diagram == nil {
		e := core.NewError(`Diagram type "`+found.Type+`" is not supported yet.`, core.CodeUnsupported)
		e.Type, e.MermaidType = found.Type, found.MermaidType
		return nil, e
	}
	config := p.Config
	if extra != nil {
		config = extra.Clone().Assign(p.Config)
	}
	source := p.Text
	if diagram.KeepComments {
		source = p.WithComments
	}
	edges := numberOr(maxEdges, defaultMaxEdges)
	var model any
	err = run(found, func() {
		model = diagram.Parse(pre.EncodeEntities(source)+"\n", config, p.Title, p.LineOffset, core.ParseLimits{MaxEdges: edges})
	})
	if err != nil {
		return nil, err
	}
	return &Parsed{Detected: found, Diagram: diagram, Model: model, Config: config}, nil
}

// Result is a rendered diagram.
type Result struct {
	core.Rendered
	Type        string
	MermaidType string
}

// Render draws a parsed diagram.
func Render(parsed *Parsed, options *core.Options) (*Result, error) {
	opts := options.Clone()
	typ := parsed.Diagram.Type
	key := parsed.Diagram.Section
	if key == "" {
		key = typ
	}
	fixed := false
	if section, ok := parsed.Config.Value(key).(*core.Object); ok {
		if v, ok := section.Value("useMaxWidth").(bool); ok && !v {
			fixed = true
		}
	}
	if opts.Responsive == nil {
		r := !fixed
		opts.Responsive = &r
	}
	// The host's icon resolver and the ids in `links` see names as the author meant them, with
	// entity codes turned into their characters, as `data-icon` and `data-id` in the SVG do.
	if resolve := options.Icons; resolve != nil {
		opts.Icons = func(name string) (string, bool) { return resolve(text.DecodeEntities(name)) }
	}
	m := opts.Measurer
	if m == nil {
		m = text.Metrics
	}
	opts.Measurer = text.WithIcons(m, opts.Icons)

	var rendered core.Rendered
	err := run(parsed.Detected, func() {
		rendered = parsed.Diagram.Render(parsed.Model, parsed.Config, opts)
	})
	if err != nil {
		return nil, err
	}
	outputLimit := numberOr(options.OutputLimit, defaultOutputLimit)
	if float64(js.Len(rendered.SVG)) > outputLimit {
		e := core.NewError("Diagram output is longer than the limit of "+js.NumberToString(outputLimit)+" characters.", core.CodeLimit)
		e.Type, e.MermaidType = parsed.Type, parsed.MermaidType
		return nil, e
	}
	for i := range rendered.Links {
		rendered.Links[i].ID = text.DecodeEntities(rendered.Links[i].ID)
	}
	if rendered.Links == nil {
		rendered.Links = []core.LinkInfo{}
	}
	return &Result{Rendered: rendered, Type: parsed.Type, MermaidType: parsed.MermaidType}, nil
}

// RenderText reads, parses and draws diagram text.
func RenderText(reg Registry, src string, options *core.Options) (*Result, error) {
	parsed, err := ParseSource(reg, src, options.Limit, options.MaxEdges, options.Config)
	if err != nil {
		return nil, err
	}
	return Render(parsed, options)
}
