package packet

import (
	"math"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Block is a field, or the piece of one in a row.
type Block struct {
	Start float64
	End   float64
	// As Mermaid counts it: one less than the width for the pieces of a field split across rows.
	Bits  float64
	Label string
}

// Model is a parsed packet diagram.
type Model struct {
	Title      string
	AccTitle   string
	AccDescr   string
	BitsPerRow float64
	Rows       [][]Block
}

const maxRows = 10_000

func packetOption(config *core.Object, key string, fallback, least, most float64) float64 {
	if value, ok := config.Section("packet").Value(key).(float64); ok && value >= least {
		return math.Min(value, most)
	}
	return fallback
}

func invalid(message string) *core.Error {
	e := core.NewError(message, core.CodeSemantic)
	e.Type = "packet"
	return e
}

// collapseIndents is .replace(/\n\s+/g, '\n').
func collapseIndents(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\n' {
			j := i + 1
			for j < len(s) {
				r, size := js.RuneAt(s, j)
				if !js.IsSpace(r) {
					break
				}
				j += size
			}
			b.WriteByte('\n')
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func buildPacket(a ast, config *core.Object, title string) *Model {
	bitsPerRow := math.Floor(packetOption(config, "bitsPerRow", 32, 1, 4096))
	var rows [][]Block
	lastBit := -1.0
	var word []Block
	row := 1.0
	n := js.NumberToString

	for _, block := range a.blocks {
		start, end, bits := block.start, block.end, block.bits
		if block.hasStart && block.hasEnd && end < start {
			panic(invalid("Packet block " + n(start) + " - " + n(end) + " is invalid. End must be greater than start."))
		}
		if !block.hasStart {
			start = lastBit + 1
		}
		if start != lastBit+1 {
			shown := start
			if block.hasEnd {
				shown = end
			}
			panic(invalid("Packet block " + n(start) + " - " + n(shown) + " is not contiguous. It should start from " + n(lastBit+1) + "."))
		}
		if block.hasBits && bits == 0 {
			panic(invalid("Packet block " + n(start) + " is invalid. Cannot have a zero bit field."))
		}
		if !block.hasEnd {
			b := 1.0
			if block.hasBits {
				b = bits
			}
			end = start + b - 1
		}
		if !block.hasBits {
			bits = end - start + 1
		}
		lastBit = end

		for float64(len(word)) <= bitsPerRow+1 && len(rows) < maxRows {
			rowEnd := row*bitsPerRow - 1
			if end <= rowEnd {
				word = append(word, Block{start, end, bits, block.label})
				if end == rowEnd {
					rows = append(rows, word)
					word = nil
					row++
				}
				break
			}
			word = append(word, Block{start, rowEnd, rowEnd - start, block.label})
			rows = append(rows, word)
			word = nil
			row++
			start = rowEnd + 1
			bits = end - start
		}
	}
	if len(word) > 0 {
		rows = append(rows, word)
	}

	m := &Model{Title: a.title, BitsPerRow: bitsPerRow, Rows: rows}
	if m.Title == "" {
		m.Title = title
	}
	if a.accTitle != "" {
		m.AccTitle = js.TrimStart(a.accTitle)
	}
	if a.accDescr != "" {
		m.AccDescr = collapseIndents(a.accDescr)
	}
	return m
}
