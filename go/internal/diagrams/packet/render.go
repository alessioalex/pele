package packet

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
	"github.com/alessioalex/pele/go/internal/svg"
	"github.com/alessioalex/pele/go/internal/text"
)

func renderPacket(model *Model, config *core.Object, options *core.Options) core.Rendered {
	return common.TurnToFit(options, func(down bool) core.Rendered { return draw(model, config, options, down) })
}

type fitKey struct {
	label string
	max   float64
}

type fitResult struct {
	fitted common.Fitted
	ok     bool
}

func draw(model *Model, config *core.Object, options *core.Options, down bool) core.Rendered {
	size := options.FontSizeOr(16)
	labelSize := js.Round(size * 0.875)
	bitSize := js.Round(size * 0.625)
	measurer := options.Measurer
	pad := options.PaddingOr(8)

	packet := config.Section("packet")
	rowHeight := packetOption(config, "rowHeight", 32, 1, 400)
	bitWidth := packetOption(config, "bitWidth", 32, 1, 400)
	paddingX := packetOption(config, "paddingX", 5, 0, 400)
	paddingY := packetOption(config, "paddingY", 5, 0, 400)
	showBits := packet.Value("showBits") != false
	descending := packet.Value("bitOrder") == "descending"
	bitsPerRow := model.BitsPerRow
	rows := model.Rows
	var labels [][]text.Label
	if down {
		maxWidth := 320.0
		if options.MaxWidth != nil {
			maxWidth = *options.MaxWidth
		}
		count := math.Max(1, math.Floor((maxWidth-2*pad)/math.Min(bitWidth, 18)))
		// A byte is the shortest row. Below that every label is one character to a line, and a
		// long packet is a row to a bit.
		bitsPerRow = math.Min(bitsPerRow, math.Max(8, js.Pow(2, math.Floor(js.Log2(count)))))
		last := 0.0
		if len(model.Rows) > 0 {
			if lastRow := model.Rows[len(model.Rows)-1]; len(lastRow) > 0 {
				last = lastRow[len(lastRow)-1].End
			}
		}
		if math.Ceil((last+1)/bitsPerRow) > 10000 {
			bitsPerRow = model.BitsPerRow
		}
		bitWidth = math.Min(bitWidth, math.Max(18, (maxWidth-2*pad)/bitsPerRow))
		// The model has split each field at its own rows. Its pieces are joined again first, so
		// that a field is only split where one of the new rows ends. A piece counts one bit short.
		piece := func(b *Block) bool { return b.Bits == b.End-b.Start }
		var whole []Block
		for _, row := range model.Rows {
			for _, block := range row {
				if len(whole) > 0 {
					prev := &whole[len(whole)-1]
					if piece(prev) && piece(&block) && prev.Label == block.Label && block.Start == prev.End+1 &&
						math.Mod(block.Start, model.BitsPerRow) == 0 {
						prev.End = block.End
						prev.Bits = prev.End - prev.Start
						continue
					}
				}
				whole = append(whole, block)
			}
		}
		rows = nil
		for _, block := range whole {
			for start := block.Start; start <= block.End; {
				end := math.Min(block.End, (math.Floor(start/bitsPerRow)+1)*bitsPerRow-1)
				index := int(math.Floor(start / bitsPerRow))
				for len(rows) <= index {
					rows = append(rows, nil)
				}
				part := block
				part.Start, part.End = start, end
				rows[index] = append(rows[index], part)
				start = end + 1
			}
		}
		labels = make([][]text.Label, len(rows))
		for i, row := range rows {
			labels[i] = make([]text.Label, len(row))
			for j, block := range row {
				room := math.Max(1, (block.End-block.Start+1)*bitWidth-math.Min(paddingX, bitWidth-1)-6)
				labels[i][j] = common.LiteralLabel(js.Trim(text.DecodeEntities(block.Label)), measurer, labelSize, room, 0)
			}
		}
	}
	// Bit numbers sit above each row and need room of their own.
	numbers := 0.0
	if showBits {
		numbers = bitSize + 4
	}
	rowHeights := make([]float64, len(rows))
	for i := range rows {
		height := rowHeight
		if down {
			for _, label := range labels[i] {
				height = math.Max(height, label.Height+10)
			}
		}
		rowHeights[i] = height
	}
	tops := make([]float64, 0, len(rows))
	rowsHeight := 0.0
	for _, height := range rowHeights {
		tops = append(tops, rowsHeight)
		rowsHeight += numbers + height + paddingY
	}
	if len(rows) > 0 {
		rowsHeight -= paddingY
	}

	gap := math.Min(paddingX, bitWidth-1)
	width := 0.0
	if len(rows) > 0 {
		width = bitsPerRow*bitWidth - gap
	}
	room := 4000.0
	if down {
		room = common.TitleRoom(options)
	}
	title := text.Layout(model.Title, false, measurer, size, room, text.Bold)
	titleHeight := 0.0
	if title.Height > 0 {
		titleHeight = title.Height
		if rowsHeight > 0 {
			titleHeight += 12
		}
	}

	// The pieces of a field split across rows share its label, so each width is fitted once.
	fits := map[fitKey]fitResult{}
	fit := func(label string, max float64) (common.Fitted, bool) {
		key := fitKey{label, max}
		if r, ok := fits[key]; ok {
			return r.fitted, r.ok
		}
		f, ok := common.FitText(js.Trim(text.DecodeEntities(label)), max, labelSize, js.Round(size*0.5), measurer, 0)
		fits[key] = fitResult{f, ok}
		return f, ok
	}

	left := pad + math.Max(0, (title.Width-width)/2)
	n := js.NumberToString
	var fields, bits strings.Builder
	for index, row := range rows {
		y := pad + tops[index] + numbers
		rowHeight := rowHeights[index]
		for j, block := range row {
			count := block.End - block.Start + 1
			first := math.Mod(block.Start, bitsPerRow)
			slot := first
			if descending {
				slot = bitsPerRow - first - count
			}
			x := left + slot*bitWidth
			w := count*bitWidth - gap
			fields.WriteString(`<g class="pele-node" data-id="` + n(block.Start) + "-" + n(block.End) + `">` +
				`<rect x="` + svg.Num(x) + `" y="` + svg.Num(y) + `" width="` + svg.Num(w) + `" height="` + svg.Num(rowHeight) +
				`" rx="` + svg.Radius + `" fill="var(--_s)" stroke="var(--_b)"/>`)
			if down {
				fields.WriteString(svg.LabelSvg(labels[index][j], x+w/2, y+rowHeight/2, "", nil))
			} else if t, ok := fit(block.Label, w-6); ok && t.Size <= rowHeight {
				fontSize := ""
				if t.Size != labelSize {
					fontSize = ` font-size="` + svg.Num(t.Size) + `"`
				}
				fields.WriteString(`<text x="` + svg.Num(x+w/2) + `" y="` + svg.Num(y+rowHeight/2+t.Size*0.35) + `"` + fontSize + ">" +
					svg.Esc(t.Text) + "</text>")
			}
			fields.WriteString("</g>")
			if !showBits {
				continue
			}
			leading, trailing := block.Start, block.End
			if descending {
				leading, trailing = block.End, block.Start
			}
			if count == 1 {
				bits.WriteString(`<text x="` + svg.Num(x+w/2) + `" y="` + svg.Num(y-4) + `" text-anchor="middle">` + n(leading) + "</text>")
			} else {
				bits.WriteString(`<text x="` + svg.Num(x) + `" y="` + svg.Num(y-4) + `">` + n(leading) + "</text>" +
					`<text x="` + svg.Num(x+w) + `" y="` + svg.Num(y-4) + `" text-anchor="end">` + n(trailing) + "</text>")
			}
		}
	}

	totalWidth := math.Ceil(math.Max(width, title.Width) + 2*pad)
	totalHeight := math.Ceil(rowsHeight + titleHeight + 2*pad)
	var content strings.Builder
	if fields.Len() > 0 {
		content.WriteString(`<g class="pele-packet-fields" font-size="` + n(labelSize) + `" text-anchor="middle">` + fields.String() + "</g>")
	}
	if bits.Len() > 0 {
		content.WriteString(`<g class="pele-packet-bits" font-size="` + n(bitSize) + `" fill="var(--_m)">` + bits.String() + "</g>")
	}
	// Mermaid puts a packet's title under it.
	content.WriteString(svg.LabelSvg(title, totalWidth/2, totalHeight-pad-title.Height/2, ` class="pele-title" font-weight="var(--_tw)"`, nil))

	acc := svg.Accessible{AccTitle: model.AccTitle, AccDescr: model.AccDescr}
	return core.Rendered{
		SVG:    svg.Document("packet", totalWidth, totalHeight, size, options, acc, content.String()),
		Width:  totalWidth,
		Height: totalHeight,
	}
}
