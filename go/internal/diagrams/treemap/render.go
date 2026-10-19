package treemap

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

const (
	header = 28
	side   = 6
	line   = 1.25
)

func option(config *core.Object, key string, fallback, min, max float64) float64 {
	if v, ok := config.Section("treemap").Value(key).(float64); ok && v >= min {
		return math.Min(v, max)
	}
	return fallback
}

func render(m *model, config *core.Object, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	tiny := js.Round(size * 0.75)
	least := js.Round(size * 0.5625)
	measurer := options.Measurer
	tm := config.Section("treemap")
	pad := options.PaddingOr(option(config, "diagramPadding", 8, 0, 2000))

	// Mermaid sizes the diagram as ten times `nodeWidth` by ten times `nodeHeight`.
	width := js.Round(common.CapWidth(options, option(config, "nodeWidth", 64, 1, 400)*10, 200, 2*pad))
	height := js.Round(option(config, "nodeHeight", 40, 1, 400) * 10)
	showValues := true
	if v, ok := tm.Value("showValues").(bool); ok && !v {
		showValues = false
	}
	spec := ","
	if v, ok := tm.Value("valueFormat").(string); ok {
		spec = v
	}
	format := formatter(spec)
	boxes := layoutTreemap(m.roots, width, height, insets{gap: option(config, "padding", 4, 0, 200), header: header, side: side})

	title := text.Layout(m.title, false, measurer, size, common.TitleRoom(options), text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 8
	}
	left := pad + math.Max(0, (title.Width-width)/2)
	top := pad + titleHeight

	var sections, leaves strings.Builder
	for _, b := range boxes {
		n := b.node
		style := svg.ResolveStyle(n.cssCompiledStyles)
		color := svg.SeriesColor(b.series)
		x := left + b.x
		y := top + b.y
		name := js.Trim(text.DecodeEntities(n.name))
		value := ""
		if showValues {
			value = format(b.value)
		}
		group := `data-id="` + svg.EscText(n.name) + `">`
		rect := `<rect x="` + svg.Num(x) + `" y="` + svg.Num(y) + `" width="` + svg.Num(b.w) + `" height="` + svg.Num(b.h) + `" rx="` + svg.Radius + `" fill="` + color + `"`

		if n.section {
			t := ""
			if b.h >= header {
				room := b.w - 2*side - 4
				valueWidth := 0.0
				if value != "" {
					valueWidth = measurer.Width(value, tiny, 0)
				}
				withValue := valueWidth > 0 && room-valueWidth-12 >= 3*small
				max := room
				if withValue {
					max = room - valueWidth - 12
				}
				label, ok := common.FitText(name, max, small, small, measurer, text.Bold)
				baseline := y + header/2 + 1
				if ok {
					t += `<text x="` + svg.Num(x+side+2) + `" y="` + svg.Num(baseline+small*0.35) + `" font-weight="var(--_hw)"` + style.Text + ">" + svg.Esc(label.Text) + "</text>"
				}
				if withValue {
					t += `<text class="pele-value" x="` + svg.Num(x+b.w-side-2) + `" y="` + svg.Num(baseline+tiny*0.35) + `" font-size="` + js.Num(tiny) + `" text-anchor="end" fill="var(--_m)"` + style.Text + ">" + svg.Esc(value) + "</text>"
				}
			}
			// A section is a tint of its color, unless a class gives it a fill of its own.
			tint := ` fill-opacity="0.12"`
			if strings.Contains(style.Shape, `"fill:`) || strings.Contains(style.Shape, ";fill:") {
				tint = ""
			}
			sections.WriteString(`<g class="pele-node pele-section` + svg.ClassNames(n.classSelector) + `" ` + group +
				rect + tint + ` stroke="` + color + `"` + style.Shape + "/>" + t + "</g>")
			continue
		}

		t := ""
		room := b.w - 8
		tallest := math.Min(size, math.Floor((b.h-4)/line))
		if tallest >= least {
			if label, ok := common.FitText(name, room, tallest, least, measurer, 0); ok {
				cx := svg.Num(x + b.w/2)
				cy := y + b.h/2
				valueSize := math.Max(least, js.Round(label.Size*0.8))
				lines := (label.Size + valueSize) * line
				if value != "" && lines <= b.h-4 && measurer.Width(value, valueSize, 0) <= room {
					first := cy - lines/2 + (label.Size*line)/2
					second := cy + lines/2 - (valueSize*line)/2
					t = `<text x="` + cx + `" y="` + svg.Num(first+label.Size*0.35) + `" font-size="` + svg.Num(label.Size) + `"` + style.Text + ">" + svg.Esc(label.Text) + "</text>" +
						`<text class="pele-value" x="` + cx + `" y="` + svg.Num(second+valueSize*0.35) + `" font-size="` + svg.Num(valueSize) + `"` + style.Text + ">" + svg.Esc(value) + "</text>"
				} else {
					t = `<text x="` + cx + `" y="` + svg.Num(cy+label.Size*0.35) + `" font-size="` + svg.Num(label.Size) + `"` + style.Text + ">" + svg.Esc(label.Text) + "</text>"
				}
			}
		}
		leaves.WriteString(`<g class="pele-node pele-leaf` + svg.ClassNames(n.classSelector) + `" ` + group + rect + style.Shape + "/>" + t + "</g>")
	}

	totalWidth := math.Ceil(math.Max(width, title.Width) + 2*pad)
	totalHeight := math.Ceil(height + titleHeight + 2*pad)
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil)
	if sections.Len() > 0 {
		content += `<g class="pele-sections" font-size="` + js.Num(small) + `">` + sections.String() + "</g>"
	}
	if leaves.Len() > 0 {
		content += `<g class="pele-leaves" text-anchor="middle" fill="var(--_bg)">` + leaves.String() + "</g>"
	}
	acc := svg.Accessible{AccTitle: m.accTitle, AccDescr: m.accDescr}
	return core.Rendered{SVG: svg.Document("treemap", totalWidth, totalHeight, size, options, acc, content), Width: totalWidth, Height: totalHeight}
}
