package core

import (
	"time"

	"github.com/alessioalex/pele/go/internal/text"
)

// Options are the TypeScript RenderOptions. A nil pointer is an option left undefined.
type Options struct {
	// Measurer measures label text. The engine always sets it before a diagram renders, wrapped
	// so that it also answers for the host's icons.
	Measurer text.Measurer
	// The fonts labels are measured in, and drawn in where the page sets no --pele-font or
	// --pele-font-mono.
	FontFamily     *string
	FontFamilyMono *string
	FontSize       *float64
	IDPrefix       *string
	Responsive     *bool
	// The width the host has for the diagram, in pixels. A type that can draw itself narrower
	// does, so that its text is not shrunk with it.
	MaxWidth *float64
	// Whether a diagram that runs across may be drawn running down when it does not fit
	// MaxWidth, and the width under which that is done.
	AutoDirection       *bool
	DirectionBreakpoint *float64
	Padding             *float64
	Limit               *float64
	OutputLimit         *float64
	// The most edges a flowchart may have.
	MaxEdges *float64
	// Schemes a link or an image may use besides a relative address. Nil means the defaults.
	LinkSchemes  []string
	ImageSchemes []string
	Links        *bool
	Images       *bool
	// Link `rel` attribute. Defaults to `noopener`; an empty string omits it.
	LinkRel *string
	Icons   text.IconResolver
	Config  *Object
	// The time gantt charts treat as now, in milliseconds since the epoch.
	Now *float64
}

// Clone copies the options one level deep.
func (o *Options) Clone() *Options {
	c := *o
	return &c
}

// FontSizeOr is `options.fontSize ?? def`.
func (o *Options) FontSizeOr(def float64) float64 {
	if o.FontSize != nil {
		return *o.FontSize
	}
	return def
}

// PaddingOr is `options.padding ?? def`.
func (o *Options) PaddingOr(def float64) float64 {
	if o.Padding != nil {
		return *o.Padding
	}
	return def
}

// IDPrefixOr is `options.idPrefix ?? def`.
func (o *Options) IDPrefixOr(def string) string {
	if o.IDPrefix != nil {
		return *o.IDPrefix
	}
	return def
}

// FontFamilyValue is options.fontFamily, with "" for undefined.
func (o *Options) FontFamilyValue() string {
	if o.FontFamily != nil {
		return *o.FontFamily
	}
	return ""
}

// FontFamilyMonoValue is options.fontFamilyMono, with "" for undefined.
func (o *Options) FontFamilyMonoValue() string {
	if o.FontFamilyMono != nil {
		return *o.FontFamilyMono
	}
	return ""
}

// Available returns options.maxWidth and whether it is a usable width: defined and above 0.
func (o *Options) Available() (float64, bool) {
	if o.MaxWidth == nil || !(*o.MaxWidth > 0) {
		return 0, false
	}
	return *o.MaxWidth, true
}

// NowOr returns options.now, or the current time.
func (o *Options) NowOr() float64 {
	if o.Now != nil {
		return *o.Now
	}
	return float64(time.Now().UnixMilli())
}

// Icon resolves a named icon through options.icons, as `options.icons?.(name)` does.
func (o *Options) Icon(name string) (string, bool) {
	if o.Icons == nil {
		return "", false
	}
	return o.Icons(name)
}

// IconOr is `options.icons?.(name) ?? def`.
func (o *Options) IconOr(name, def string) string {
	if markup, ok := o.Icon(name); ok {
		return markup
	}
	return def
}

// LinkInfo describes a link drawn in the SVG.
type LinkInfo struct {
	ID       string
	Href     string
	Internal bool
}

// Rendered is what a diagram type draws.
type Rendered struct {
	SVG    string
	Width  float64
	Height float64
	Links  []LinkInfo
}

// ParseLimits are limits set by the host. A diagram's own configuration cannot change them.
type ParseLimits struct {
	MaxEdges float64
}

// Diagram is what a diagram type provides. The source given to Parse has been through
// preprocessing and entity encoding and ends with a newline, as Mermaid's parsers expect. Parse
// and Render may panic with *Error; the engine turns that into a returned error.
type Diagram struct {
	Type string
	// The name of this type's section in Mermaid's config, when it is not the type's own name.
	Section string
	// A diagram that reports source positions is given the text with its `%%` comment lines
	// kept. lineOffset is the number of front matter lines that were removed before that text.
	KeepComments bool
	Parse        func(source string, config *Object, title string, lineOffset int, limits ParseLimits) any
	Render       func(model any, config *Object, options *Options) Rendered
}

// Version is the version of the TypeScript library this port follows.
const Version = "1.7.0"
