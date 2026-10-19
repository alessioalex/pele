// Package pele renders Mermaid diagrams to SVG. It is a Go port of the Pele TypeScript library
// and draws exactly what that library draws: the same SVG, themed with the same CSS custom
// properties, for every diagram type built into Mermaid 12.1.0.
//
//	res, err := pele.Render("flowchart LR\n  A[Mermaid text] --> B[SVG]", nil)
//	if err != nil { ... }
//	os.WriteFile("diagram.svg", []byte(res.SVG), 0o644)
//
// Rendering is synchronous, deterministic and safe for concurrent use.
package pele

import (
	"math"
	"slices"
	"sort"
	"time"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/engine"
	"github.com/alessioalex/pele/go/internal/pre"
	"github.com/alessioalex/pele/go/internal/text"
)

// TextMeasurer measures label text: the width in pixels of text drawn at size pixels in a style
// made of StyleBold, StyleItalic and StyleMono. Labels are laid out with it, so the drawing fits
// the text best when it measures in the font the SVG is shown in.
type TextMeasurer interface {
	Width(text string, size float64, style int) float64
}

// Style bits given to a TextMeasurer.
const (
	StyleBold   = text.Bold
	StyleItalic = text.Italic
	StyleMono   = text.Mono
)

// MetricsMeasurer estimates widths from a table of Arial advance widths. It is the default, and
// what the TypeScript library uses where it has no canvas.
var MetricsMeasurer TextMeasurer = text.Metrics

// Options configure Render. The zero value of each field means its default.
type Options struct {
	// Measurer measures labels. Nil means MetricsMeasurer.
	Measurer TextMeasurer
	// The fonts labels are measured in, and drawn in where the page sets no --pele-font or
	// --pele-font-mono. Empty means sans-serif and monospace.
	FontFamily     string
	FontFamilyMono string
	// FontSize is the base font size in pixels. 0 means 16.
	FontSize float64
	// IDPrefix prefixes the ids in the SVG, so that several diagrams can share a page. Empty
	// means "pele".
	IDPrefix string
	// Responsive makes the SVG scale down to its container (max-width:100%). Nil means true,
	// unless the diagram's config sets useMaxWidth: false.
	Responsive *bool
	// MaxWidth is the width the host has for the diagram, in pixels. A type that can draw itself
	// narrower does, so that its text is not shrunk with it. 0 means no limit.
	MaxWidth float64
	// AutoDirection lets a diagram that runs across be drawn running down when it does not fit
	// MaxWidth and MaxWidth is under DirectionBreakpoint. Nil means true; 0 means 640 pixels.
	AutoDirection       *bool
	DirectionBreakpoint float64
	// Padding around the diagram in pixels. Nil means each type's default.
	Padding *float64
	// Limit is the longest source accepted, in UTF-16 code units. 0 means 50,000; negative
	// means no limit.
	Limit int
	// OutputLimit is the longest SVG returned, in UTF-16 code units. 0 means 4,000,000;
	// negative means no limit.
	OutputLimit int
	// MaxEdges is the most edges a flowchart may have. 0 means 5,000; negative means no limit.
	MaxEdges int
	// Schemes a link or an image may use besides a relative address. Nil means http, https,
	// mailto and tel for links, and http and https for images.
	LinkSchemes  []string
	ImageSchemes []string
	// NoLinks and NoImages leave links and images out of the drawing.
	NoLinks  bool
	NoImages bool
	// LinkRel is the `rel` of links. Nil means "noopener"; an empty string omits it.
	LinkRel *string
	// Icons resolves an icon name such as "fa:fa-user" to SVG markup drawn in a 24×24 box.
	// It returns false when it has no such icon. Nil means no icons.
	Icons func(name string) (markup string, ok bool)
	// Config is Mermaid configuration, as `%%{init: ...}%%` would give it. The diagram's own
	// front matter and directives win over it.
	Config map[string]any
	// Now is the time gantt charts treat as now. The zero value means the current time.
	Now time.Time
}

// Bool returns a pointer to b, for the optional boolean options.
func Bool(b bool) *bool { return &b }

// Float returns a pointer to f, for Padding.
func Float(f float64) *float64 { return &f }

// String returns a pointer to s, for LinkRel.
func String(s string) *string { return &s }

// Link is a link drawn in the SVG.
type Link struct {
	// ID is the id of the element the link is on, as the diagram names it.
	ID       string `json:"id"`
	Href     string `json:"href"`
	Internal bool   `json:"internal"`
}

// Result is a rendered diagram.
type Result struct {
	SVG    string  `json:"svg"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Links  []Link  `json:"links"`
	// Type is Pele's name for the diagram type, such as "flowchart" or "gitGraph".
	Type string `json:"type"`
	// MermaidType is the type as Mermaid names it, such as "flowchart-v2" for `graph TD`.
	MermaidType string `json:"mermaidType"`
}

// Error is the error Render and Parse return for text they cannot draw. Line and Column are
// 1-based and 0 when unknown; columns count UTF-16 code units, as in the TypeScript library.
type Error = core.Error

// ErrorCode classifies an Error.
type ErrorCode = core.ErrorCode

const (
	// ErrUnsupported: no diagram type was detected.
	ErrUnsupported = core.CodeUnsupported
	// ErrSyntax: the text does not parse.
	ErrSyntax = core.CodeSyntax
	// ErrSemantic: the text parses but describes something that cannot be drawn.
	ErrSemantic = core.CodeSemantic
	// ErrLimit: the source, the drawing or the nesting exceeds a limit.
	ErrLimit = core.CodeLimit
)

var registry = engine.NewRegistry(all...)

// limit turns a limit option into the TypeScript's: undefined for the default, Infinity for none.
func limit(v int) *float64 {
	if v == 0 {
		return nil
	}
	f := float64(v)
	if v < 0 {
		f = math.Inf(1)
	}
	return &f
}

func configObject(m map[string]any) *core.Object {
	o := core.NewObject()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		o.Set(k, configValue(m[k]))
	}
	return o
}

func configValue(v any) core.Value {
	switch x := v.(type) {
	case nil, bool, string, float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float32:
		return float64(x)
	case map[string]any:
		return configObject(x)
	case []any:
		out := make([]core.Value, len(x))
		for i, e := range x {
			out[i] = configValue(e)
		}
		return out
	case []string:
		out := make([]core.Value, len(x))
		for i, e := range x {
			out[i] = e
		}
		return out
	}
	return nil
}

func (o *Options) core() *core.Options {
	c := &core.Options{}
	if o == nil {
		return c
	}
	if o.Measurer != nil {
		c.Measurer = o.Measurer
	}
	str := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	num := func(f float64) *float64 {
		if f == 0 {
			return nil
		}
		return &f
	}
	c.FontFamily = str(o.FontFamily)
	c.FontFamilyMono = str(o.FontFamilyMono)
	c.FontSize = num(o.FontSize)
	c.IDPrefix = str(o.IDPrefix)
	c.Responsive = o.Responsive
	c.MaxWidth = num(o.MaxWidth)
	c.AutoDirection = o.AutoDirection
	c.DirectionBreakpoint = num(o.DirectionBreakpoint)
	c.Padding = o.Padding
	c.Limit = limit(o.Limit)
	c.OutputLimit = limit(o.OutputLimit)
	c.MaxEdges = limit(o.MaxEdges)
	c.LinkSchemes = slices.Clone(o.LinkSchemes)
	c.ImageSchemes = slices.Clone(o.ImageSchemes)
	if o.NoLinks {
		c.Links = Bool(false)
	}
	if o.NoImages {
		c.Images = Bool(false)
	}
	c.LinkRel = o.LinkRel
	if o.Icons != nil {
		c.Icons = text.IconResolver(o.Icons)
	}
	if o.Config != nil {
		c.Config = configObject(o.Config)
	}
	if !o.Now.IsZero() {
		ms := float64(o.Now.UnixMilli())
		c.Now = &ms
	}
	return c
}

func result(r *engine.Result) *Result {
	links := make([]Link, len(r.Links))
	for i, l := range r.Links {
		links[i] = Link{ID: l.ID, Href: l.Href, Internal: l.Internal}
	}
	return &Result{SVG: r.SVG, Width: r.Width, Height: r.Height, Links: links, Type: r.Type, MermaidType: r.MermaidType}
}

// Render draws Mermaid diagram text as SVG. Options may be nil. Text that cannot be drawn gives
// an *Error.
func Render(diagram string, options *Options) (*Result, error) {
	r, err := engine.RenderText(registry, diagram, options.core())
	if err != nil {
		return nil, err
	}
	return result(r), nil
}

// Model is diagram text that was parsed, to be drawn any number of times, at any width.
type Model struct {
	parsed *engine.Parsed
}

// Type is Pele's name for the diagram type.
func (m *Model) Type() string { return m.parsed.Type }

// MermaidType is the diagram type as Mermaid names it.
func (m *Model) MermaidType() string { return m.parsed.MermaidType }

// Parse reads diagram text without drawing it. Only the Limit, MaxEdges and Config options are
// read; options may be nil.
func Parse(diagram string, options *Options) (*Model, error) {
	c := options.core()
	p, err := engine.ParseSource(registry, diagram, c.Limit, c.MaxEdges, c.Config)
	if err != nil {
		return nil, err
	}
	return &Model{parsed: p}, nil
}

// Render draws a parsed diagram. The options that are read when the text is parsed (Limit,
// MaxEdges, Config) have no effect here.
func (m *Model) Render(options *Options) (*Result, error) {
	r, err := engine.Render(m.parsed, options.core())
	if err != nil {
		return nil, err
	}
	return result(r), nil
}

// DetectType returns the diagram type of the text, or "" when none is detected or its front
// matter is not valid YAML.
func DetectType(diagram string) string {
	p, err := pre.Preprocess(diagram)
	if err != nil {
		return ""
	}
	found, ok := pre.DetectDiagram(p.Text)
	if !ok {
		return ""
	}
	return found.Type
}

// Supports reports whether the text is of a diagram type Pele draws.
func Supports(diagram string) bool {
	t := DetectType(diagram)
	return t != "" && registry[t] != nil
}

// Types lists the diagram types Pele draws.
func Types() []string {
	out := make([]string, 0, len(registry))
	for t := range registry {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
