package treeview

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
	iconGap           = 6
	descGap           = 16
	guideInset        = 7
	elbow             = 4
	highlightOverhang = 4
)

func setting(config *core.Object, key string, fallback, lo, hi float64) float64 {
	if value, ok := config.Section("treeView").Value(key).(float64); ok && value >= lo && value <= hi {
		return value
	}
	return fallback
}

type treeSettings struct {
	paddingX, paddingY, rowIndent, lineThickness float64
}

// settingsOf reads Mermaid's layout options, with Pele's defaults.
func settingsOf(config *core.Object) treeSettings {
	return treeSettings{
		paddingX:      setting(config, "paddingX", 5, 0, 100),
		paddingY:      setting(config, "paddingY", 2, 0, 100),
		rowIndent:     setting(config, "rowIndent", 15, 0, 400),
		lineThickness: setting(config, "lineThickness", 1, 0.25, 12),
	}
}

type row struct {
	node        *TreeNode
	x           float64
	cy          float64
	icon        string
	text        string
	labelX      float64
	highlighted bool
	label       *text.Label
	description *text.Label
	top         float64
	height      float64
}

func renderTreeView(model *Model, config *core.Object, options *core.Options) core.Rendered {
	return common.Tighten(options, func(tight bool) core.Rendered { return draw(model, config, options, tight) })
}

func maxWidthOr(options *core.Options, def float64) float64 {
	if options.MaxWidth != nil {
		return *options.MaxWidth
	}
	return def
}

func styleOf(node *TreeNode) int {
	if node.Directory {
		return text.Bold
	}
	return 0
}

func draw(model *Model, config *core.Object, options *core.Options, tight bool) core.Rendered {
	size := options.FontSizeOr(16)
	measurer := options.Measurer
	pad := options.PaddingOr(8)
	icons := iconConfig(config)

	s := settingsOf(config)
	paddingX, paddingY, rowIndent, lineThickness := s.paddingX, s.paddingY, s.rowIndent, s.lineThickness
	maxDepth := 0
	type entry struct {
		node  *TreeNode
		depth int
	}
	if tight {
		depths := make([]entry, 0, len(model.Root.Children))
		for _, node := range model.Root.Children {
			depths = append(depths, entry{node, 0})
		}
		for i := 0; i < len(depths); i++ {
			e := depths[i]
			maxDepth = max(maxDepth, e.depth)
			for _, child := range e.node.Children {
				depths = append(depths, entry{child, e.depth + 1})
			}
		}
	}
	step := rowIndent + paddingX
	if tight {
		step = math.Min(rowIndent+paddingX, math.Max(2, (maxWidthOr(options, 320)-120)/float64(max(1, maxDepth))))
	}
	rowHeight := js.Round(size*1.5) + 2*paddingY
	iconSize := size

	titleWidth := 4000.0
	if tight {
		titleWidth = common.TitleRoom(options)
	}
	title := text.Layout(model.Title, false, measurer, size, titleWidth, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}

	// Siblings line up: when one entry of a folder has an icon, the others leave room for one.
	hasIcon := func(list []*TreeNode) bool {
		for _, child := range list {
			if getNodeIcon(child, icons) != "" {
				return true
			}
		}
		return false
	}
	var rows []*row
	rowOf := map[*TreeNode]*row{}
	type item struct {
		node  *TreeNode
		depth int
		slot  bool
	}
	var pending []item
	push := func(list []*TreeNode, depth int) {
		slot := hasIcon(list)
		for i := len(list) - 1; i >= 0; i-- {
			pending = append(pending, item{list[i], depth, slot})
		}
	}
	push(model.Root.Children, 0)
	labelRight := 0.0
	anyHighlight := false
	rowsHeight := 0.0
	for len(pending) > 0 {
		it := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		node := it.node
		x := float64(it.depth) * step
		label := text.DecodeEntities(node.Name)
		labelX := x
		if it.slot {
			labelX += iconSize + iconGap
		}
		room := math.Max(40, maxWidthOr(options, 320)-2*pad-2*highlightOverhang-labelX)
		var lbl, description *text.Label
		if tight {
			l := common.LiteralLabel(label, measurer, size, room, styleOf(node))
			lbl = &l
			if node.Description != "" {
				d := common.LiteralLabel(text.DecodeEntities(node.Description), measurer, size, room, text.Italic)
				description = &d
			}
		}
		var width float64
		if lbl != nil {
			width = lbl.Width
		} else {
			width = measurer.Width(label, size, styleOf(node))
		}
		if labelX+width > labelRight {
			labelRight = labelX + width
		}
		highlighted := false
		if node.CSSClass != "" {
			for _, c := range strings.FieldsFunc(node.CSSClass, js.IsSpace) {
				if c == "highlight" {
					highlighted = true
				}
			}
		}
		if highlighted {
			anyHighlight = true
		}
		height := rowHeight
		var cy float64
		if tight {
			lh, dh := 0.0, 0.0
			if lbl != nil {
				lh = lbl.Height
			}
			if description != nil {
				dh = description.Height
			}
			height = math.Max(rowHeight, lh+dh+2*paddingY)
			cy = rowsHeight + (paddingY + math.Max(iconSize, lh)/2)
		} else {
			cy = rowsHeight + height/2
		}
		r := &row{node: node, x: x, cy: cy, icon: getNodeIcon(node, icons), text: label, labelX: labelX, highlighted: highlighted,
			label: lbl, description: description, top: rowsHeight, height: height}
		rowsHeight += height
		rows = append(rows, r)
		rowOf[node] = r
		push(node.Children, it.depth+1)
	}

	descX := labelRight + descGap
	width := labelRight
	for _, r := range rows {
		description := r.node.Description
		if description == "" {
			continue
		}
		var w float64
		if r.description != nil {
			w = r.description.Width
		} else {
			w = measurer.Width(text.DecodeEntities(description), size, text.Italic)
		}
		at := descX
		if tight {
			at = r.labelX
		}
		width = math.Max(width, at+w)
	}
	width = math.Max(width, title.Width)
	inset := pad
	if anyHighlight {
		inset += highlightOverhang
	}
	top := pad + titleHeight
	totalWidth := math.Ceil(width + 2*inset)
	totalHeight := math.Ceil(top + rowsHeight + pad)

	var guides strings.Builder
	for _, r := range rows {
		children := r.node.Children
		if len(children) == 0 {
			continue
		}
		gx := inset + r.x + guideInset
		tick := math.Max(gx, inset+r.x+step-paddingX)
		last := top + rowOf[children[len(children)-1]].cy
		rr := math.Min(elbow, tick-gx)
		d := "M" + svg.Num(gx) + "," + svg.Num(top+r.top+r.height-2) + "V" + svg.Num(last-rr)
		if rr > 0 {
			d += "Q" + svg.Num(gx) + "," + svg.Num(last) + " " + svg.Num(gx+rr) + "," + svg.Num(last) + "H" + svg.Num(tick)
		}
		var more strings.Builder
		for i := 0; i < len(children)-1 && tick > gx; i++ {
			more.WriteString("M" + svg.Num(gx) + "," + svg.Num(top+rowOf[children[i]].cy) + "H" + svg.Num(tick))
		}
		guides.WriteString(`<path d="` + d + more.String() + `"/>`)
	}

	baseline := size * 0.35
	var body strings.Builder
	for _, r := range rows {
		node := r.node
		cy := top + r.cy
		directory := node.Directory
		var inner strings.Builder
		if r.highlighted {
			x := inset + r.x - highlightOverhang
			inner.WriteString(`<rect class="pele-tree-highlight" x="` + svg.Num(x) + `" y="` + svg.Num(top+r.top+1) + `" width="` +
				svg.Num(totalWidth-inset+highlightOverhang-x) + `" height="` + svg.Num(r.height-2) + `" rx="` + svg.Radius +
				`" fill="var(--_c)" fill-opacity="0.14"/>`)
		}
		if r.icon != "" {
			name := ""
			if strings.HasPrefix(r.icon, builtinPack+":") {
				name = r.icon[len(builtinPack)+1:]
			}
			glyph, ok := glyphs[name]
			if !ok {
				if directory {
					glyph = glyphs["folder"]
				} else {
					glyph = glyphs["file"]
				}
			}
			drawn := options.IconOr(r.icon, `<path d="`+glyph+`" stroke="var(--_m)" stroke-width="1.5"/>`)
			inner.WriteString(`<svg class="pele-icon" data-icon="` + svg.EscText(r.icon) + `" x="` + svg.Num(inset+r.x) + `" y="` + svg.Num(cy-iconSize/2) +
				`" width="` + svg.Num(iconSize) + `" height="` + svg.Num(iconSize) +
				`" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + drawn + "</svg>")
		}
		weight := ""
		if directory {
			weight = ` font-weight="var(--_hw)"`
		}
		if r.label != nil {
			inner.WriteString(common.LeftLabelSvg(*r.label, inset+r.labelX, top+r.top+paddingY, ` class="pele-label"`+weight))
		} else if r.text != "" {
			inner.WriteString(`<text class="pele-label" x="` + svg.Num(inset+r.labelX) + `" y="` + svg.Num(cy+baseline) + `"` + weight +
				` xml:space="preserve">` + svg.Esc(r.text) + "</text>")
		}
		if r.description != nil {
			lh := 0.0
			if r.label != nil {
				lh = r.label.Height
			}
			inner.WriteString(common.LeftLabelSvg(*r.description, inset+r.labelX, top+r.top+paddingY+lh, ` class="pele-tree-description" font-style="italic" fill="var(--_m)"`))
		} else if node.Description != "" {
			inner.WriteString(`<text class="pele-tree-description" x="` + svg.Num(inset+descX) + `" y="` + svg.Num(cy+baseline) +
				`" font-style="italic" fill="var(--_m)" xml:space="preserve">` + svg.Esc(text.DecodeEntities(node.Description)) + "</text>")
		}
		kind := "file"
		if directory {
			kind = "dir"
		}
		body.WriteString(`<g class="pele-node pele-tree-` + kind + svg.ClassNames(node.CSSClass) + `" data-id="` + svg.EscText(node.Name) + `">` + inner.String() + "</g>")
	}

	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil)
	if guides.Len() > 0 {
		content += `<g class="pele-tree-guides" fill="none" stroke="var(--_b)" stroke-width="` + svg.Num(lineThickness) + `">` + guides.String() + "</g>"
	}
	content += body.String()

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("treeView", totalWidth, totalHeight, size, options, acc, content),
		Width:  totalWidth,
		Height: totalHeight,
	}
}
