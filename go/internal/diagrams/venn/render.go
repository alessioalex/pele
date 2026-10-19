package venn

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

const (
	radius    = 100
	maxGrowth = 1.6
)

type line struct {
	// id is that of a text; nil for the region's own label.
	id    *string
	raw   string
	size  float64
	base  int
	label text.Label
}

type region struct {
	sets   []string
	inside []int
	spot   Spot
	// What is written in the region: its label first, then its texts.
	lines []line
	style svg.ResolvedStyle
}

// keyOf is one key for a list of set ids. Ids may hold any character, so they are not simply joined.
func keyOf(ids []string) string {
	if len(ids) == 1 {
		return ids[0]
	}
	return setsKey(ids)
}

func renderVenn(model *DB, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	tiny := js.Round(size * 0.8125)
	measurer := options.Measurer
	pad := options.PaddingOr(8)

	circles := layoutCircles(model.Subsets)
	index := map[string]int{}
	for i, c := range circles {
		index[c.ID] = i
	}

	// Styles for the same targets add up, later declarations winning.
	styles := map[string]*orderedMap{}
	for _, s := range model.Styles {
		key := keyOf(s.Targets)
		merged := styles[key]
		if merged == nil {
			merged = &orderedMap{}
			styles[key] = merged
		}
		for _, property := range s.Styles.keys {
			merged.set(property, s.Styles.vals[property])
		}
	}
	styleOf := func(key string) svg.ResolvedStyle {
		declared := styles[key]
		if declared == nil {
			return svg.ResolvedStyle{}
		}
		list := make([]string, 0, len(declared.keys))
		for _, property := range declared.keys {
			list = append(list, property+":"+declared.vals[property])
		}
		return svg.ResolveStyle(list)
	}

	// Circles that touch each circle; only those can cut into a region of it.
	neighbors := make([][]int, len(circles))
	for a := 0; a < len(circles); a++ {
		for b := a + 1; b < len(circles); b++ {
			if js.Hypot(circles[a].X-circles[b].X, circles[a].Y-circles[b].Y) < circles[a].R+circles[b].R {
				neighbors[a] = append(neighbors[a], b)
				neighbors[b] = append(neighbors[b], a)
			}
		}
	}

	// One region for each set and each stated overlap; a later statement about the same sets
	// replaces an earlier one.
	var statedKeys []string
	stated := map[string]*Subset{}
	for i := range model.Subsets {
		key := keyOf(model.Subsets[i].Sets)
		if _, ok := stated[key]; !ok {
			statedKeys = append(statedKeys, key)
		}
		stated[key] = &model.Subsets[i]
	}
	regions := make([]*region, 0, len(statedKeys))
	byKey := make(map[string]*region, len(statedKeys))
	for _, key := range statedKeys {
		subset := stated[key]
		var sets []string
		seen := map[string]bool{}
		for _, id := range subset.Sets {
			if !seen[id] {
				seen[id] = true
				sets = append(sets, id)
			}
		}
		inside := make([]int, len(sets))
		within := map[int]bool{}
		for i, id := range sets {
			inside[i] = index[id]
			within[inside[i]] = true
		}
		smallest := inside[0]
		for _, i := range inside {
			if circles[i].R < circles[smallest].R {
				smallest = i
			}
		}
		style := styleOf(key)
		label := ""
		hasText := false
		if subset.Label != nil {
			label, hasText = *subset.Label, true
		} else if len(subset.Sets) == 1 {
			label, hasText = subset.Sets[0], true
		}
		base := 0
		if style.Bold {
			base |= text.Bold
		}
		if style.Italic {
			base |= text.Italic
		}
		var outside []int
		for _, i := range neighbors[smallest] {
			if !within[i] {
				outside = append(outside, i)
			}
		}
		spot := findSpot(circles, inside, outside)
		// A region that other circles cover, or all but cover, is written across them instead.
		if spot.Margin < circles[smallest].R*0.2 {
			spot = findSpot(circles, inside, nil)
		}
		r := &region{sets: sets, inside: inside, spot: spot, style: style}
		if hasText && label != "" {
			fontSize := small
			if len(sets) == 1 {
				fontSize = size
			}
			r.lines = append(r.lines, line{raw: label, size: fontSize, base: base, label: text.Layout(label, false, measurer, fontSize, 4000, base)})
		}
		regions = append(regions, r)
		byKey[key] = r
	}
	for _, node := range model.TextNodes {
		r := byKey[keyOf(node.Sets)]
		if r == nil {
			continue
		}
		raw := node.ID
		if node.Label != nil {
			raw = *node.Label
		}
		id := node.ID
		r.lines = append(r.lines, line{id: &id, raw: raw, size: tiny, label: text.Layout(raw, false, measurer, tiny, 4000, 0)})
	}

	// Lengths so far are in units of the layout. The scale gives the largest circle a set radius,
	// and grows, within limits, until every region holds what is written in it.
	largest := 0.0
	for _, c := range circles {
		if c.R > largest {
			largest = c.R
		}
	}
	scale := 1.0
	if largest > 0 {
		scale = radius / largest
	}
	growth := 1.0
	for _, r := range regions {
		if r.spot.Margin <= 0 {
			continue
		}
		width := 0.0
		height := 0.0
		for _, l := range r.lines {
			width = math.Max(width, l.label.Width)
			height += l.label.Height
		}
		need := js.Hypot(width, height)/2 + 4
		growth = math.Max(growth, math.Min(need/(r.spot.Margin*scale), maxGrowth))
	}
	scale *= growth

	// What is still too wide for its region is wrapped to it.
	for _, r := range regions {
		room := math.Max(r.spot.Margin*scale*1.8, 72)
		for i := range r.lines {
			l := &r.lines[i]
			if l.label.Width > room {
				l.label = text.Layout(l.raw, false, measurer, l.size, room, l.base)
			}
		}
	}

	minX := math.Inf(1)
	minY := math.Inf(1)
	maxX := math.Inf(-1)
	maxY := math.Inf(-1)
	for _, c := range circles {
		minX = math.Min(minX, (c.X-c.R)*scale)
		minY = math.Min(minY, (c.Y-c.R)*scale)
		maxX = math.Max(maxX, (c.X+c.R)*scale)
		maxY = math.Max(maxY, (c.Y+c.R)*scale)
	}
	for _, r := range regions {
		wide := 0.0
		tall := 0.0
		for _, l := range r.lines {
			wide = math.Max(wide, l.label.Width)
			tall += l.label.Height
		}
		minX = math.Min(minX, r.spot.X*scale-wide/2)
		maxX = math.Max(maxX, r.spot.X*scale+wide/2)
		minY = math.Min(minY, r.spot.Y*scale-tall/2)
		maxY = math.Max(maxY, r.spot.Y*scale+tall/2)
	}
	if len(circles) == 0 {
		minX, minY, maxX, maxY = 0, 0, 0, 0
	}

	titleText := model.Title
	if titleText != "" {
		titleText = normalizeText(titleText)
	}
	title := text.Layout(titleText, false, measurer, size, 4000, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 12
	}
	inner := math.Max(maxX-minX, title.Width)
	ox := pad + (inner-(maxX-minX))/2 - minX
	oy := pad + titleHeight - minY
	width := math.Ceil(inner + 2*pad)
	height := math.Ceil(maxY - minY + titleHeight + 2*pad)
	placed := make([]Circle, len(circles))
	for i, c := range circles {
		placed[i] = Circle{ID: c.ID, X: c.X*scale + ox, Y: c.Y*scale + oy, R: c.R * scale}
	}

	var sets strings.Builder
	for i := range placed {
		c := &placed[i]
		color := svg.SeriesColor(i)
		shape := ""
		if r := byKey[c.ID]; r != nil {
			shape = r.style.Shape
		}
		sets.WriteString(`<circle class="pele-venn-set" data-id="` + svg.EscText(c.ID) + `" cx="` + svg.Num(c.X) + `" cy="` + svg.Num(c.Y) + `" r="` + svg.Num(c.R) +
			`" fill="` + color + `" fill-opacity="0.2" stroke="` + color + `"` + shape + "/>")
	}

	var overlaps, labels strings.Builder
	for _, r := range regions {
		id := svg.EscText(strings.Join(r.sets, "|"))
		// An overlap is only drawn when a style gives it a look of its own; otherwise the circles show it.
		if len(r.sets) > 1 && r.style.Shape != "" {
			shared := make([]*Circle, len(r.inside))
			for i, k := range r.inside {
				shared[i] = &placed[k]
			}
			if d := regionPath(shared, svg.Num); d != "" {
				overlaps.WriteString(`<path class="pele-venn-overlap" data-id="` + id + `" d="` + d + `" fill="none"` + r.style.Shape + "/>")
			}
		}
		total := 0.0
		for _, l := range r.lines {
			total += l.label.Height
		}
		if total == 0 {
			continue
		}
		x := r.spot.X*scale + ox
		y := r.spot.Y*scale + oy - total/2
		var content strings.Builder
		for _, l := range r.lines {
			var attrs string
			if l.id == nil {
				attrs = ` class="pele-label"`
				if l.size != size {
					attrs += ` font-size="` + js.NumberToString(l.size) + `"`
				}
				attrs += r.style.Text
			} else {
				attrs = ` class="pele-venn-text" data-id="` + svg.EscText(*l.id) + `" font-size="` + js.NumberToString(tiny) + `" fill="var(--_m)"` + styleOf(*l.id).Text
			}
			content.WriteString(svg.LabelSvg(l.label, x, y+l.label.Height/2, attrs, nil))
			y += l.label.Height
		}
		labels.WriteString(`<g class="pele-venn-region" data-id="` + id + `">` + content.String() + "</g>")
	}

	body := svg.LabelSvg(title, width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) +
		`<g class="pele-venn-sets">` + sets.String() + "</g>"
	if overlaps.Len() > 0 {
		body += `<g class="pele-venn-overlaps">` + overlaps.String() + "</g>"
	}
	if labels.Len() > 0 {
		body += `<g class="pele-venn-labels">` + labels.String() + "</g>"
	}
	doc := svg.Document("venn", width, height, size, options, svg.Accessible{}, body)
	return core.Rendered{SVG: doc, Width: width, Height: height}
}
