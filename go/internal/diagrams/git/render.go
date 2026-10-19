package git

import (
	"math"
	"slices"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

const (
	dot  = 7.0
	step = 44.0
	lane = 44.0
	// Shorter than a step, so a curve between lanes never runs through a commit on a lane it crosses.
	curve    = 36.0
	gap      = 8.0
	labelGap = 20.0
	tail     = 20.0
	notch    = 7.0
	tagGap   = 6.0
	half     = 0.7071067811865476 // Math.SQRT1_2
)

var classes = []string{"normal", "reverse", "highlight", "merge", "cherry-pick"}

type view struct {
	commit *Commit
	index  int
	lane   int
	m      float64
	label  *text.Label
	tags   []text.Label
}

func flag(config *core.Object, key string, fallback bool) bool {
	if v, ok := config.Section("gitGraph").Value(key).(bool); ok {
		return v
	}
	return fallback
}

func tagWidth(tag text.Label) float64 { return notch + 4 + tag.Width + 6 }

func symbol(typ int, x, y float64, color string) string {
	at := `cx="` + svg.Num(x) + `" cy="` + svg.Num(y) + `"`
	if typ == typeHighlight {
		return `<rect x="` + svg.Num(x-dot-1) + `" y="` + svg.Num(y-dot-1) + `" width="` + js.Num(2*dot+2) + `" height="` + js.Num(2*dot+2) + `" rx="2" fill="` + color + `"/>`
	}
	if typ == typeMerge {
		return `<circle ` + at + ` r="` + js.Num(dot-1) + `" fill="var(--_bg)" stroke="` + color + `" stroke-width="2"/><circle ` + at + ` r="2.5" fill="` + color + `"/>`
	}
	d := `<circle ` + at + ` r="` + js.Num(dot) + `" fill="` + color + `"/>`
	if typ == typeReverse {
		return d + `<path d="M` + svg.Num(x-3) + `,` + svg.Num(y-3) + `l6,6M` + svg.Num(x-3) + `,` + svg.Num(y+3) + `l6,-6" fill="none" stroke="var(--_bg)" stroke-width="1.5" stroke-linecap="round"/>`
	}
	if typ == typeCherryPick {
		return d +
			`<path d="M` + svg.Num(x-2.6) + `,` + svg.Num(y+1) + `L` + svg.Num(x+0.8) + `,` + svg.Num(y-4.2) + `L` + svg.Num(x+2.6) + `,` + svg.Num(y+1) + `" fill="none" stroke="var(--_bg)" stroke-linejoin="round"/>` +
			`<circle cx="` + svg.Num(x-2.6) + `" cy="` + svg.Num(y+2.2) + `" r="1.8" fill="var(--_bg)"/>` +
			`<circle cx="` + svg.Num(x+2.6) + `" cy="` + svg.Num(y+2.2) + `" r="1.8" fill="var(--_bg)"/>`
	}
	return d
}

func renderGit(model *Model, config *core.Object, options *core.Options) core.Rendered {
	if model.Direction != "LR" {
		return draw(model, config, options, model.Direction)
	}
	return common.TurnToFit(options, func(down bool) core.Rendered {
		if down {
			return draw(model, config, options, "TB")
		}
		return draw(model, config, options, "LR")
	})
}

func draw(model *Model, config *core.Object, options *core.Options, direction string) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.75)
	measurer := options.Measurer
	pad := options.PaddingOr(8)

	showBranches := flag(config, "showBranches", true)
	showCommitLabel := flag(config, "showCommitLabel", true)
	rotate := flag(config, "rotateCommitLabel", true)
	parallel := flag(config, "parallelCommits", false)
	vertical := direction != "LR"
	flip := direction == "BT"

	laneOf := map[string]int{}
	for i, n := range model.Lanes {
		laneOf[n] = i
	}
	laneCount := len(model.Lanes)
	medium := js.Round(size * 0.875)
	// Mermaid writes branch names, tags, and commit ids as plain text, so tags in them show as written.
	names := make([]text.Label, laneCount)
	for i, n := range model.Lanes {
		if !showBranches {
			n = ""
		}
		names[i] = common.PlainLabel(n, measurer, medium, 4000, 0)
	}

	sorted := make([]*Commit, 0, len(model.ids))
	for _, id := range model.ids {
		sorted = append(sorted, model.Commits[id])
	}
	slices.SortStableFunc(sorted, func(a, b *Commit) int { return a.Seq - b.Seq })

	views := make([]*view, 0, len(sorted))
	byID := make(map[string]*view, len(sorted))
	for _, c := range sorted {
		labelled := showCommitLabel && c.Type != typeCherryPick && (c.Type != typeMerge || c.CustomID)
		var tags []text.Label
		for _, tag := range c.Tags {
			l := common.PlainLabel(tag, measurer, small, 4000, 0)
			if len(l.Lines) > 0 {
				tags = append(tags, l)
			}
		}
		v := &view{commit: c, index: len(views), lane: laneOf[c.Branch], tags: tags}
		if labelled {
			l := common.PlainLabel(c.ID, measurer, small, 4000, 0)
			v.label = &l
		}
		views = append(views, v)
		byID[c.ID] = v
	}

	// Room each commit needs: along the main axis on either side, and across it towards the
	// previous lane (`lo`) and the next one (`hi`). Tags sit on one side of a lane and ids on the other.
	halfs := make([]float64, len(views))
	tagHalf := make([]float64, len(views))
	lo := make([]float64, laneCount)
	hi := make([]float64, laneCount)
	for i := range lo {
		lo[i] = dot
		hi[i] = dot
	}
	for k, v := range views {
		tagsMain, tagsCross := 0.0, 0.0
		for _, tag := range v.tags {
			w := tagWidth(tag)
			if vertical {
				tagsMain = max(tagsMain, tag.Height)
				tagsCross += w + 4
			} else {
				tagsMain = max(tagsMain, w)
				tagsCross += tag.Height + 3
			}
		}
		labelMain, labelCross := 0.0, 0.0
		if v.label != nil && len(v.label.Lines) > 0 {
			if rotate {
				labelCross = 3 + half*(v.label.Width+v.label.Height)
			} else if vertical {
				labelMain = v.label.Height
				labelCross = 6 + v.label.Width
			} else {
				labelMain = v.label.Width
				labelCross = 6 + v.label.Height
			}
		}
		halfs[k] = max(dot, tagsMain/2, labelMain/2)
		tagHalf[k] = tagsMain / 2
		tagSide := dot
		if len(v.tags) > 0 {
			tagSide = dot + (tagGap + tagsCross)
		}
		labelSide := dot + labelCross
		l := v.lane
		if vertical {
			lo[l] = max(lo[l], labelSide)
			hi[l] = max(hi[l], tagSide)
		} else {
			lo[l] = max(lo[l], tagSide)
			hi[l] = max(hi[l], labelSide)
		}
	}

	// In a left-to-right graph the tags stand over their commit. A connector that comes down from
	// a lane above joins the commit's lane this far before the commit, clear of the tags, and the
	// commit is placed far enough along to leave room for that.
	lead := make([]float64, len(views))
	if !vertical {
		for k, v := range views {
			if tagHalf[k] == 0 {
				continue
			}
			for _, id := range v.commit.Parents {
				if p := byID[id]; p != nil && p.lane < v.lane {
					lead[k] = tagHalf[k] + 4
				}
			}
		}
	}
	space := func(p, k int) float64 {
		extra := 0.0
		if lead[k] > 0 {
			extra = halfs[p] + curve + lead[k]
		}
		return max(step, halfs[p]+halfs[k]+gap, extra)
	}

	for k, v := range views {
		if !parallel {
			if k == 0 {
				v.m = 0
			} else {
				v.m = views[k-1].m + space(k-1, k)
			}
			continue
		}
		// Mermaid's `parallelCommits`: a commit sits one step after its furthest parent, so branches
		// that grow side by side share positions instead of taking turns.
		m := 0.0
		for _, id := range v.commit.Parents {
			p := byID[id]
			if p == nil || p.index >= k {
				continue
			}
			m = max(m, p.m+space(p.index, k))
		}
		v.m = m
	}
	end := 0.0
	for _, v := range views {
		end = max(end, v.m)
	}

	cross := make([]float64, laneCount)
	for i := 0; i < laneCount; i++ {
		n := names[i]
		reach := 0.0
		if len(n.Lines) > 0 {
			if vertical {
				reach = n.Width/2 + 8
			} else {
				reach = n.Height/2 + 2
			}
		}
		lo[i] = max(lo[i], reach)
		hi[i] = max(hi[i], reach)
		if i > 0 {
			cross[i] = cross[i-1] + max(lane, hi[i-1]+lo[i]+gap)
		}
	}

	px := func(m, c float64) float64 {
		if vertical {
			return c
		}
		return m
	}
	py := func(m, c float64) float64 {
		if !vertical {
			return c
		}
		if flip {
			return -m
		}
		return m
	}
	point := func(m, c float64) string { return svg.Num(px(m, c)) + "," + svg.Num(py(m, c)) }

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	grow := func(x0, y0, x1, y1 float64) {
		if x0 < minX {
			minX = x0
		}
		if y0 < minY {
			minY = y0
		}
		if x1 > maxX {
			maxX = x1
		}
		if y1 > maxY {
			maxY = y1
		}
	}

	var branches strings.Builder
	if showBranches {
		for i := 0; i < laneCount; i++ {
			n := names[i]
			color := svg.SeriesColor(i)
			c := cross[i]
			w := n.Width + 16
			h := n.Height + 4
			along := w
			if vertical {
				along = h
			}
			from := -labelGap
			if len(n.Lines) > 0 {
				from = -labelGap - along
			}
			cx := px(from+along/2, c)
			cy := py(from+along/2, c)
			grow(px(-labelGap, c), py(-labelGap, c), px(-labelGap, c), py(-labelGap, c))
			grow(px(end+tail, c), py(end+tail, c), px(end+tail, c), py(end+tail, c))
			branches.WriteString(`<g class="pele-branch" data-id="` + svg.EscText(model.Lanes[i]) + `">` +
				`<path class="pele-lane" d="M` + point(-labelGap, c) + `L` + point(end+tail, c) + `" stroke="` + color + `" stroke-dasharray="2 4"/>`)
			if len(n.Lines) > 0 {
				grow(cx-w/2, cy-h/2, cx+w/2, cy+h/2)
				branches.WriteString(`<rect x="` + svg.Num(cx-w/2) + `" y="` + svg.Num(cy-h/2) + `" width="` + svg.Num(w) + `" height="` + svg.Num(h) + `" rx="` + svg.Radius + `" fill="` + color + `"/>` +
					svg.LabelSvg(n, cx, cy, ` class="pele-label" fill="var(--_bg)"`, nil))
			}
			branches.WriteString("</g>")
		}
	}

	// Positions of the commits on each lane, to tell whether a connector may run along it.
	taken := make([][]float64, laneCount)
	for _, v := range views {
		taken[v.lane] = append(taken[v.lane], v.m)
	}
	if parallel {
		for _, list := range taken {
			slices.SortStableFunc(list, func(a, b float64) int { return js.Cmp(a - b) })
		}
	}
	busy := func(l int, from, to float64) bool {
		list := taken[l]
		low, high := 0, len(list)
		for low < high {
			mid := (low + high) >> 1
			if list[mid] <= from {
				low = mid + 1
			} else {
				high = mid
			}
		}
		return low < len(list) && list[low] < to
	}

	var curves, lines strings.Builder
	for _, v := range views {
		c := v.commit
		c2 := cross[v.lane]
		m2 := v.m
		for j, pid := range c.Parents {
			parent := byID[pid]
			if parent == nil || parent == v {
				continue
			}
			c1 := cross[parent.lane]
			m1 := parent.m
			if parent.lane == v.lane {
				lines.WriteString(`<path class="pele-edge" d="M` + point(m1, c1) + `L` + point(m2, c2) + `" stroke="` + svg.SeriesColor(v.lane) + `"/>`)
				continue
			}
			merged := j > 0 && c.Type == typeMerge
			picked := j > 0 && !merged
			var d strings.Builder
			d.WriteString("M" + point(m1, c1))
			if m2 <= m1 {
				mid := (m1 + m2) / 2
				d.WriteString("C" + point(mid, c1) + " " + point(mid, c2) + " " + point(m2, c2))
			} else if !merged && !busy(v.lane, m1, m2) {
				// Leaves the parent at once and runs along the child's lane.
				s := math.Min(curve, m2-m1)
				d.WriteString("C" + point(m1+s/2, c1) + " " + point(m1+s/2, c2) + " " + point(m1+s, c2) + "L" + point(m2, c2))
			} else {
				// Turns in just before the child, or before its tags when it comes from their side.
				to := m2
				if parent.lane < v.lane && m2-lead[v.index] > m1 {
					to = m2 - lead[v.index]
				}
				s := math.Min(curve, to-m1)
				along := c1
				if !merged || busy(parent.lane, m1, m2) {
					// Neither lane is free to run along. Between two lanes, past the ids of one and before
					// the tags of the next, a strip stays empty, and the connector takes that.
					if c1 > c2 {
						along = c2 + hi[v.lane] + gap/2
					} else {
						along = c2 - lo[v.lane] - gap/2
					}
					s = math.Min(curve, (to-m1)/2)
					d.WriteString("C" + point(m1+s/2, c1) + " " + point(m1+s/2, along) + " " + point(m1+s, along))
				}
				d.WriteString("L" + point(to-s, along) + "C" + point(to-s/2, along) + " " + point(to-s/2, c2) + " " + point(to, c2))
				if to < m2 {
					d.WriteString("L" + point(m2, c2))
				}
			}
			class := ""
			if merged {
				class = " pele-edge-merge"
			} else if picked {
				class = " pele-edge-cherry-pick"
			}
			colorLane := v.lane
			if merged {
				colorLane = parent.lane
			}
			dash := ""
			if picked {
				dash = ` stroke-dasharray="4 3"`
			}
			curves.WriteString(`<path class="pele-edge` + class + `" d="` + d.String() + `" ` +
				`stroke="` + svg.SeriesColor(colorLane) + `"` + dash + `/>`)
		}
	}

	var commits strings.Builder
	for _, v := range views {
		c := v.commit
		x := px(v.m, cross[v.lane])
		y := py(v.m, cross[v.lane])
		typ := c.Type
		if c.CustomType >= 0 {
			typ = c.CustomType
		}
		grow(x-dot-1, y-dot-1, x+dot+1, y+dot+1)
		var body strings.Builder
		// Merges and cherry-picks carry a message Mermaid makes up; only a written one is worth showing.
		if c.Type < typeMerge && c.Message != "" {
			body.WriteString("<title>" + svg.EscText(c.Message) + "</title>")
		}
		body.WriteString(symbol(typ, x, y, svg.SeriesColor(v.lane)))

		if label := v.label; label != nil && len(label.Lines) > 0 {
			w := label.Width
			h := label.Height
			back := func(cx, cy float64) string {
				return `<rect x="` + svg.Num(cx-w/2-2) + `" y="` + svg.Num(cy-h/2+2) + `" width="` + svg.Num(w+4) + `" height="` + svg.Num(h-4) + `" rx="3" fill="var(--_bg)"/>`
			}
			if rotate {
				// The text ends beside the commit and trails back the way the history came.
				k := (half * h) / 2
				ax, ay := x+3, y+dot+3+k
				if vertical {
					ax, ay = x-dot-3-k, y
				}
				reach := half*w + k
				if flip {
					grow(ax-reach, ay-reach, ax+k, ay+k)
				} else {
					grow(ax-reach, ay-k, ax+k, ay+reach)
				}
				angle := "-45"
				if flip {
					angle = "45"
				}
				body.WriteString(`<g class="pele-commit-label" transform="rotate(` + angle + ` ` + svg.Num(ax) + ` ` + svg.Num(ay) + `)">` +
					back(ax-w/2, ay) +
					svg.LabelSvg(*label, ax-w/2, ay, "", nil) +
					"</g>")
			} else {
				cx, cy := x, y+dot+6+h/2
				if vertical {
					cx, cy = x-dot-6-w/2, y
				}
				grow(cx-w/2, cy-h/2, cx+w/2, cy+h/2)
				body.WriteString(`<g class="pele-commit-label">` + back(cx, cy) + svg.LabelSvg(*label, cx, cy, "", nil) + "</g>")
			}
		}

		// Beside the lane the tags form a row; above it they stack, the first one furthest out.
		offset := dot + tagGap
		t, dt := len(v.tags)-1, -1
		if vertical {
			t, dt = 0, 1
		}
		for ; t >= 0 && t < len(v.tags); t += dt {
			tag := v.tags[t]
			w := tagWidth(tag)
			h := tag.Height
			var left, cy float64
			if vertical {
				left, cy = x+offset, y
				offset += w + 4
			} else {
				left, cy = x-w/2, y-offset-h/2
				offset += h + 3
			}
			grow(left, cy-h/2, left+w, cy+h/2)
			body.WriteString(`<g class="pele-tag">` +
				`<path d="M` + svg.Num(left) + `,` + svg.Num(cy) + `L` + svg.Num(left+notch) + `,` + svg.Num(cy-h/2) + `H` + svg.Num(left+w) + `V` + svg.Num(cy+h/2) + `H` + svg.Num(left+notch) + `Z" fill="var(--_s)" stroke="var(--_b)" stroke-linejoin="round"/>` +
				`<circle cx="` + svg.Num(left+notch) + `" cy="` + svg.Num(cy) + `" r="1.5" fill="var(--_b)"/>` +
				svg.LabelSvg(tag, left+notch+4+tag.Width/2, cy, ` fill="var(--_fg)"`, nil) +
				"</g>")
		}

		merge := ""
		if c.Type == typeMerge && typ != typeMerge {
			merge = " pele-commit-merge"
		}
		commits.WriteString(`<g class="pele-commit pele-commit-` + classes[typ] + merge + `" data-id="` + svg.EscText(c.ID) + `">` + body.String() + "</g>")
	}

	if minX > maxX {
		grow(0, 0, 0, 0)
	}
	title := text.Layout(model.Title, false, measurer, size, 4000, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 16
	}
	inner := max(maxX-minX, title.Width)
	width := math.Ceil(inner + 2*pad)
	height := math.Ceil(maxY - minY + titleHeight + 2*pad)
	dx := pad - minX + (inner-(maxX-minX))/2
	dy := pad + titleHeight - minY

	var content strings.Builder
	content.WriteString(svg.LabelSvg(title, width/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil))
	content.WriteString(`<g transform="translate(` + svg.Num(dx) + `,` + svg.Num(dy) + `)">`)
	if branches.Len() > 0 {
		content.WriteString(`<g class="pele-branches" fill="none" font-size="` + js.Num(medium) + `">` + branches.String() + "</g>")
	}
	if curves.Len() > 0 || lines.Len() > 0 {
		content.WriteString(`<g class="pele-edges" fill="none" stroke-width="2" stroke-linecap="round">` + curves.String() + lines.String() + "</g>")
	}
	if commits.Len() > 0 {
		content.WriteString(`<g class="pele-commits" font-size="` + js.Num(small) + `" fill="var(--_m)">` + commits.String() + "</g>")
	}
	content.WriteString("</g>")

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("gitGraph", width, height, size, options, acc, content.String()),
		Width:  width,
		Height: height,
	}
}
