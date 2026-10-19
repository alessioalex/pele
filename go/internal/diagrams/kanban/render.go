package kanban

import (
	"math"
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
	"github.com/alessioalex/pele/go/internal/util"
)

const (
	columnGap = 12
	columnPad = 8
	cardPad   = 10
	cardGap   = 8
	metaGap   = 4
	iconGap   = 6
)

// Mermaid names five priorities and gives Medium no color. Matching here ignores case. The high
// ones are marked in the critical color and the low ones in the muted one, the nearer to Medium
// the fainter. They are not series colors, as a theme may put those in any order.
var priorityColors = map[string]string{
	"very high": `stroke="` + svg.Critical + `"`,
	"high":      `stroke="` + svg.Critical + `" opacity="0.5"`,
	"low":       `stroke="var(--_m)" opacity="0.5"`,
	"very low":  `stroke="var(--_m)"`,
}

// fitLabel lays out text for a fixed width. A word too long for a line is broken between
// characters, because a card cannot grow sideways to hold it.
func fitLabel(raw string, markdown bool, m text.Measurer, size, max float64, base int) text.Label {
	label := text.Layout(raw, markdown, m, size, max, base)
	if label.Width <= max {
		return label
	}
	var lines [][]text.Span
	var widths []float64
	widest := 0.0
	var line []text.Span
	width := 0.0
	commit := func() {
		lines = append(lines, line)
		widths = append(widths, width)
		widest = math.Max(widest, width)
		line = nil
		width = 0
	}
	for i := range label.Lines {
		if label.Widths[i] <= max {
			line = label.Lines[i]
			width = label.Widths[i]
			commit()
			continue
		}
		for _, span := range label.Lines[i] {
			if span.Icon != "" {
				if width > 0 && width+span.Width > max {
					commit()
				}
				line = append(line, span)
				width += span.Width
				continue
			}
			start := 0
			from := width
			for k, ch := range span.Text {
				w := m.Width(string(ch), size, span.Style)
				if width > 0 && width+w > max {
					if k > start {
						line = append(line, text.Span{Text: span.Text[start:k], Style: span.Style, Width: width - from})
					}
					commit()
					start = k
					from = 0
				}
				width += w
			}
			if len(span.Text) > start {
				line = append(line, text.Span{Text: span.Text[start:], Style: span.Style, Width: width - from})
			}
		}
		commit()
	}
	label.Lines = lines
	label.Widths = widths
	label.Width = widest
	label.Height = float64(len(lines)) * label.LineHeight
	return label
}

// textBlock writes a label with every line starting at x, or ending there, and the first line's
// box starting at y.
func textBlock(label text.Label, x, y float64, attrs string, icons text.IconResolver, end bool) string {
	var out strings.Builder
	for i, line := range label.Lines {
		cy := y + (float64(i)+0.5)*label.LineHeight
		if len(line) == 1 && line[0].Icon == "" {
			anchor := ""
			if end {
				anchor = ` text-anchor="end"`
			}
			out.WriteString("<text" + svg.TextStyle(attrs, line[0].Style) + ` x="` + svg.Num(x) + `" y="` + svg.Num(cy+label.Size*0.35) + `"` +
				anchor + ">" + svg.Esc(line[0].Text) + "</text>")
		} else {
			width := label.Widths[i]
			one := label
			one.Lines = [][]text.Span{line}
			one.Widths = []float64{width}
			one.Width = width
			one.Height = label.LineHeight
			shift := width
			if end {
				shift = -width
			}
			out.WriteString(svg.LabelSvg(one, x+shift/2, cy, attrs, icons))
		}
	}
	return out.String()
}

type card struct {
	item     *Node
	label    text.Label
	ticket   text.Label
	assigned text.Label
	// Height of the rows under the text that hold the ticket and the assignee.
	meta   float64
	height float64
}

func renderKanban(model *Model, config *core.Object, options *core.Options) core.Rendered {
	size := options.FontSizeOr(16)
	small := js.Round(size * 0.875)
	line := js.Round(size * 1.5)
	smallLine := js.Round(small * 1.5)
	m := options.Measurer
	pad := options.PaddingOr(8)
	icons := options.Icons
	has := func(name string) bool { return name != "" && text.IconMarkup(m, name) != "" }
	conf := config.Section("kanban")
	ownWidth := 200.0
	if sw, ok := conf.Value("sectionWidth").(float64); ok && sw > 0 {
		ownWidth = math.Min(math.Max(sw, 80), 2000)
	}
	// A board wider than the width the host has is folded into rows of columns, which then
	// share the row between them.
	room := math.Inf(1)
	if options.MaxWidth != nil && *options.MaxWidth > 0 {
		room = *options.MaxWidth - 2*pad
	}
	sections := model.Sections
	count := float64(len(sections))
	fits := count*(ownWidth+columnGap)-columnGap <= room
	var perRowF, columnWidth float64
	if fits {
		perRowF = math.Max(count, 1)
		columnWidth = ownWidth
	} else {
		perRowF = math.Max(1, math.Floor((room+columnGap)/(ownWidth+columnGap)))
		columnWidth = math.Max(math.Min(ownWidth, room), math.Min(math.Floor((room+columnGap)/perRowF)-columnGap, ownWidth*2))
	}
	// At most the number of columns, as it is only below that when they do not fit.
	perRow := int(perRowF)
	baseURL, _ := conf.Value("ticketBaseUrl").(string)
	cardWidth := columnWidth - 2*columnPad
	textWidth := cardWidth - 2*cardPad
	glyph := small * 1.25
	var links []core.LinkInfo

	titles := make([]text.Label, 0, len(sections))
	countWidths := make([]float64, 0, len(sections))
	cards := make([][]card, 0, len(sections))
	headHeight := line
	bodyHeights := make([]float64, 0, len(sections))
	for _, section := range sections {
		countWidth := m.Width(strconv.Itoa(len(section.Items)), small, 0)
		reserved := countWidth + 12
		if has(section.Icon) {
			reserved += glyph + iconGap
		}
		title := fitLabel(section.Label, true, m, size, cardWidth-8-reserved, text.Bold)
		headHeight = math.Max(headHeight, title.Height)
		countWidths = append(countWidths, countWidth)
		titles = append(titles, title)

		list := make([]card, 0, len(section.Items))
		height := 0.0
		for _, item := range section.Items {
			room := textWidth
			if has(item.Icon) {
				room -= glyph + iconGap
			}
			label := fitLabel(item.Label, true, m, small, room, 0)
			ticket := fitLabel(item.Ticket, false, m, small, textWidth, 0)
			assigned := fitLabel(item.Assigned, false, m, small, textWidth, 0)
			var meta float64
			if ticket.Width+assigned.Width+8 > textWidth {
				meta = ticket.Height + assigned.Height
			} else {
				meta = math.Max(ticket.Height, assigned.Height)
			}
			cardHeight := 2*cardPad + math.Max(label.Height, smallLine)
			if meta != 0 && !math.IsNaN(meta) {
				cardHeight += metaGap + meta
			}
			list = append(list, card{item: item, label: label, ticket: ticket, assigned: assigned, meta: meta, height: cardHeight})
			height += cardHeight + cardGap
		}
		cards = append(cards, list)
		bodyHeights = append(bodyHeights, height)
	}
	headHeight += 2 * columnPad
	var rowHeights, rowTops []float64
	y := 0.0
	for first := 0; first < len(sections); first += perRow {
		tallest := float64(cardGap)
		for index := first; index < min(first+perRow, len(sections)); index++ {
			tallest = math.Max(tallest, bodyHeights[index])
		}
		rowTops = append(rowTops, y)
		rowHeights = append(rowHeights, headHeight+tallest)
		y += headHeight + tallest + columnGap
	}
	boardHeight := 0.0
	if len(rowHeights) > 0 {
		boardHeight = rowTops[len(rowTops)-1] + rowHeights[len(rowHeights)-1]
	}

	title := text.Layout(model.Title, false, m, size, common.TitleRoom(options), text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height + 16
	}
	boardWidth := math.Max(math.Min(count, perRowF)*(columnWidth+columnGap)-columnGap, 0)
	width := math.Max(boardWidth, title.Width)
	left := pad + (width-boardWidth)/2
	edge := columnWidth - columnPad - 4
	smallText := js.NumberToString(small)

	var out strings.Builder
	for index, section := range sections {
		row := index / perRow
		var body strings.Builder
		body.WriteString(`<rect width="` + js.NumberToString(columnWidth) + `" height="` + svg.Num(rowHeights[row]) + `" rx="` + svg.Radius + `" fill="var(--_s)"/>` +
			textBlock(titles[index], columnPad+4, columnPad, ` class="pele-cluster-label" font-weight="var(--_hw)"`, icons, false) +
			`<text class="pele-count" x="` + js.NumberToString(edge) + `" y="` + svg.Num(columnPad+line/2+small*0.35) +
			`" text-anchor="end" font-size="` + smallText + `" fill="var(--_m)">` + strconv.Itoa(len(section.Items)) + "</text>")
		if has(section.Icon) {
			body.WriteString(common.IconSvg(section.Icon, edge-countWidths[index]-iconGap-glyph, columnPad+(line-glyph)/2, glyph, icons))
		}

		y := headHeight
		for _, c := range cards[index] {
			item := c.item
			priority := js.ToLower(js.Trim(item.Priority))
			color, colored := priorityColors[priority]
			var inner strings.Builder
			inner.WriteString(`<rect width="` + js.NumberToString(cardWidth) + `" height="` + svg.Num(c.height) + `" rx="` + svg.Radius + `" fill="var(--_bg)" stroke="var(--_b)"/>`)
			if colored {
				inner.WriteString(`<path class="pele-priority" d="M2.5,5V` + svg.Num(c.height-5) + `" fill="none" ` + color + ` stroke-width="3" stroke-linecap="round"/>`)
			}
			inner.WriteString(textBlock(c.label, cardPad, cardPad, ` class="pele-label"`, icons, false))
			if has(item.Icon) {
				inner.WriteString(common.IconSvg(item.Icon, cardWidth-cardPad-glyph, cardPad+(smallLine-glyph)/2, glyph, icons))
			}
			bottom := c.height - cardPad
			if item.Ticket != "" {
				top := bottom - c.meta
				href, linked := "", false
				if baseURL != "" {
					href, linked = util.LinkURL(util.SafeURL(strings.Replace(baseURL, "#TICKET#", item.Ticket, 1)), options)
				}
				if !linked {
					inner.WriteString(textBlock(c.ticket, cardPad, top, ` class="pele-ticket" fill="var(--_m)"`, icons, false))
				} else {
					inner.WriteString(`<a href="` + svg.Esc(href) + `" target="_blank"` + util.RelAttr(options) + ">" +
						textBlock(c.ticket, cardPad, top, ` class="pele-ticket" fill="var(--_c)"`, icons, false) +
						"</a>")
					links = append(links, core.LinkInfo{ID: item.ID, Href: href, Internal: false})
				}
			}
			if item.Assigned != "" {
				inner.WriteString(textBlock(c.assigned, cardWidth-cardPad, bottom-c.assigned.Height, ` class="pele-assigned" fill="var(--_m)"`, icons, true))
			}
			class := ""
			if colored {
				class = " pele-priority-" + strings.Replace(priority, " ", "-", 1)
			}
			body.WriteString(`<g class="pele-node pele-card` + class + svg.ClassNames(item.CSSClasses) + `" data-id="` + svg.EscText(item.ID) +
				`" transform="translate(` + strconv.Itoa(columnPad) + "," + svg.Num(y) + `)" font-size="` + smallText + `">` + inner.String() + "</g>")
			y += c.height + cardGap
		}

		out.WriteString(`<g class="pele-cluster pele-column` + svg.ClassNames(section.CSSClasses) + `" data-id="` + svg.EscText(section.ID) +
			`" transform="translate(` + svg.Num(left+float64(index%perRow)*(columnWidth+columnGap)) + "," + svg.Num(pad+titleHeight+rowTops[row]) + `)">` +
			body.String() + "</g>")
	}

	totalWidth := math.Ceil(width + 2*pad)
	totalHeight := math.Ceil(boardHeight + titleHeight + 2*pad)
	content := svg.LabelSvg(title, totalWidth/2, pad+title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil) + out.String()

	return core.Rendered{
		SVG:    svg.Document("kanban", totalWidth, totalHeight, size, options, svg.Accessible{}, content),
		Width:  totalWidth,
		Height: totalHeight,
		Links:  links,
	}
}
