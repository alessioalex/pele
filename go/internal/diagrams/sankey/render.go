package sankey

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
	labelGap = 6.0
	valueGap = 15.0
)

func numberOption(conf *core.Object, key string, fallback, lo, hi float64) float64 {
	if v, ok := conf.Value(key).(float64); ok && v >= lo {
		return math.Min(v, hi)
	}
	return fallback
}

func textOption(conf *core.Object, key string) string {
	switch v := conf.Value(key).(type) {
	case string:
		return v
	case float64:
		if js.IsFinite(v) {
			return js.NumberToString(v)
		}
	}
	return ""
}

// Two decimals, as Mermaid shows values; numbers too large for that are left as they are.
func rounded(value float64) string {
	if math.Abs(value) < 1e15 {
		return js.NumberToString(js.Round(value*100) / 100)
	}
	return js.NumberToString(value)
}

// A colour from the configuration, as an inline style that overrides the series colour.
func fillStyle(color core.Value) string {
	if s, ok := color.(string); ok {
		return svg.ResolveStyle([]string{"fill:" + s}).Shape
	}
	return ""
}

func renderSankey(model *Model, config *core.Object, options *core.Options) core.Rendered {
	conf := config.Section("sankey")
	return common.FitWidth(options, numberOption(conf, "width", 600, 1, 10000), 160, func(chartWidth float64, _ bool) core.Rendered {
		return draw(model, conf, options, chartWidth)
	})
}

func draw(model *Model, conf *core.Object, options *core.Options, chartWidth float64) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	measurer := options.Measurer
	pad := options.PaddingOr(8)

	chartHeight := numberOption(conf, "height", 400, 1, 10000)
	nodeWidth := math.Min(numberOption(conf, "nodeWidth", 10, 1, 200), chartWidth)
	showValues := conf.Value("showValues") != false
	prefix := textOption(conf, "prefix")
	suffix := textOption(conf, "suffix")
	outlined := conf.Value("labelStyle") == "outlined"
	alignment := "justify"
	switch a := conf.Value("nodeAlignment"); a {
	case "left", "right", "center":
		alignment = a.(string)
	}
	linkColor := "gradient"
	if s, ok := conf.Value("linkColor").(string); ok {
		linkColor = s
	}

	padding := numberOption(conf, "nodePadding", 12, 0, 200)
	if showValues {
		padding += valueGap
	}
	nodes, links := layoutSankey(model, layoutOptions{
		width:       chartWidth,
		height:      chartHeight,
		nodeWidth:   nodeWidth,
		nodePadding: padding,
		labelHeight: small * 1.15,
		alignment:   alignment,
	})

	custom := map[string]string{}
	if colors := conf.Section("nodeColors"); colors != nil {
		for _, id := range colors.Keys() {
			if style := fillStyle(colors.Value(id)); style != "" {
				custom[id] = style
			}
		}
	}
	nodeStyle := func(i int) string { return custom[model.Nodes[i].ID] }

	// Labels sit beside the bars, on the side that faces the middle of the chart.
	central := 0
	most := 0.0
	for _, node := range nodes {
		if node.value > most {
			most = node.value
			central = node.layer
		}
	}
	minX, maxX := 0.0, chartWidth
	minY, maxY := 0.0, chartHeight
	var labelsOut, nodesOut strings.Builder
	for i, node := range nodes {
		id := model.Nodes[i].ID
		height := math.Max(node.height, 1)
		nodesOut.WriteString(`<rect class="pele-sankey-node" data-id="` + svg.EscText(id) + `" x="` + svg.Num(node.x) + `" y="` + svg.Num(node.y) + `" width="` +
			svg.Num(nodeWidth) + `" height="` + svg.Num(height) + `" rx="` + svg.Num(math.Min(2, height/2)) + `" fill="` + svg.SeriesColor(i) + `"` + nodeStyle(i) + `/>`)

		value := ""
		if showValues {
			value = prefix + rounded(node.value) + suffix
		}
		after := node.x < chartWidth/2
		if outlined {
			after = node.layer >= central
		}
		label := id
		if value != "" {
			label = id + " " + value
		}
		textWidth := measurer.Width(label, small, 0)
		x := node.x - labelGap
		if after {
			x = node.x + nodeWidth + labelGap
		}
		y := node.y + node.height/2
		if after {
			minX = math.Min(minX, x)
			maxX = math.Max(maxX, x+textWidth)
		} else {
			minX = math.Min(minX, x-textWidth)
			maxX = math.Max(maxX, x)
		}
		minY = math.Min(minY, y-small*0.75)
		maxY = math.Max(maxY, y+small*0.75)
		anchor := "end"
		if after {
			anchor = "start"
		}
		position := ` x="` + svg.Num(x) + `" y="` + svg.Num(y+small*0.35) + `" text-anchor="` + anchor + `"`
		body := svg.Esc(id)
		if value != "" {
			body += ` <tspan class="pele-sankey-value" fill="var(--_m)">` + svg.Esc(value) + `</tspan>`
		}
		labelsOut.WriteString(`<g class="pele-label" data-id="` + svg.EscText(id) + `">`)
		if outlined {
			plain := svg.Esc(id)
			if value != "" {
				plain += " " + svg.Esc(value)
			}
			labelsOut.WriteString(`<text` + position + ` fill="none" stroke="var(--_bg)" stroke-width="3" stroke-linejoin="round">` + plain + `</text>`)
		}
		labelsOut.WriteString(`<text` + position + `>` + body + `</text></g>`)
	}

	fixed := ""
	if linkColor != "gradient" && linkColor != "source" && linkColor != "target" {
		fixed = fillStyle(linkColor)
	}
	var linksOut strings.Builder
	for _, link := range links {
		// Without definitions there are no gradients, so a gradient link takes its source's colour.
		end := link.source
		if linkColor == "target" {
			end = link.target
		}
		half := math.Max(link.thickness, 1) / 2
		x0 := nodes[link.source].x + nodeWidth
		x1 := nodes[link.target].x
		mid := svg.Num((x0 + x1) / 2)
		from := svg.EscText(model.Nodes[link.source].ID)
		to := svg.EscText(model.Nodes[link.target].ID)
		style := fixed
		if style == "" {
			style = nodeStyle(end)
		}
		linksOut.WriteString(`<path class="pele-sankey-link" data-id="` + from + `-&gt;` + to + `" d="M` + svg.Num(x0) + `,` + svg.Num(link.sourceY-half) +
			`C` + mid + `,` + svg.Num(link.sourceY-half) + ` ` + mid + `,` + svg.Num(link.targetY-half) + ` ` + svg.Num(x1) + `,` + svg.Num(link.targetY-half) +
			`L` + svg.Num(x1) + `,` + svg.Num(link.targetY+half) +
			`C` + mid + `,` + svg.Num(link.targetY+half) + ` ` + mid + `,` + svg.Num(link.sourceY+half) + ` ` + svg.Num(x0) + `,` + svg.Num(link.sourceY+half) +
			`Z" fill="` + svg.SeriesColor(end) + `"` + style + `>` +
			`<title>` + from + ` → ` + to + `: ` + svg.EscText(prefix) + rounded(link.value) + svg.EscText(suffix) + `</title></path>`)
	}

	title := text.Layout(model.Title, false, measurer, size, common.TitleRoom(options), text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	inner := math.Max(maxX-minX, title.Width)
	ox := pad - minX + (inner-(maxX-minX))/2
	oy := pad + titleHeight - minY
	width := math.Ceil(inner + 2*pad)
	height := math.Ceil(maxY - minY + titleHeight + 2*pad)

	content := svg.LabelSvg(title, width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) +
		`<g transform="translate(` + svg.Num(ox) + `,` + svg.Num(oy) + `)">` +
		`<g class="pele-sankey-links" fill-opacity="0.4">` + linksOut.String() + `</g>` +
		`<g class="pele-sankey-nodes">` + nodesOut.String() + `</g>` +
		`<g class="pele-labels" font-size="` + js.Num(small) + `">` + labelsOut.String() + `</g>` +
		`</g>`

	return core.Rendered{
		SVG:    svg.Document("sankey", width, height, size, options, svg.Accessible{}, content),
		Width:  width,
		Height: height,
	}
}
