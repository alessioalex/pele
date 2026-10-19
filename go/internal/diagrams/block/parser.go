package block

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

type nodeInfo struct {
	id         string
	label      string
	typeStr    string
	directions []string
}

type frame struct {
	list   []*Block
	header *statement
}

const statementStart = "'NODE_ID', 'COLUMNS', 'SPACE_BLOCK', 'BLOCK_DIAGRAM_KEY', 'id-block', 'classDef', 'class', 'style'"

// appendStatement adds a statement to a list as `.flat()` would read it, which is how every list
// of statements is used.
func appendStatement(list []*Block, s statement) []*Block {
	if s.chain != nil {
		return append(list, s.chain...)
	}
	return append(list, s.block)
}

func parseBlock(src string, db *blockDb) {
	toks := tokenize(src)
	types, texts, starts := toks.types, toks.texts, toks.starts
	i := 0

	fail := func(expected string) {
		t := types[i]
		if t == tERROR {
			panic(core.SyntaxError("block", src, starts[i], "", true))
		}
		panic(core.SyntaxError("block", src, starts[i], "Expecting "+expected+", got '"+tokenNames[t]+"'", false))
	}

	expect := func(typ int) string {
		if types[i] != typ {
			fail("'" + tokenNames[typ] + "'")
		}
		i++
		return texts[i-1]
	}

	node := func() nodeInfo {
		id := expect(tNODE_ID)
		if types[i] == tNODE_DSTART {
			open := texts[i]
			i++
			label := expect(tSTR)
			return nodeInfo{id: id, label: label, typeStr: open + expect(tNODE_DEND)}
		}
		if types[i] == tBLOCK_ARROW_START {
			open := texts[i]
			i++
			label := expect(tSTR)
			dirs := []string{expect(tDIR)}
			for types[i] == tDIR {
				dirs = append(dirs, texts[i])
				i++
			}
			return nodeInfo{id: id, label: label, typeStr: open + expect(tBLOCK_ARROW_END), directions: dirs}
		}
		return nodeInfo{id: id}
	}

	nodeStatement := func() statement {
		first := node()
		size := 1.0
		if types[i] == tSIZE {
			size = js.ParseInt(texts[i], 10)
			i++
		}
		from := &Block{ID: first.id, Label: first.label, Type: typeStr2Type(first.typeStr), Directions: first.directions, WidthInColumns: size, Columns: -1}
		var chain []*Block
		for types[i] == tLINK || types[i] == tSTART_LINK {
			label := ""
			i++
			if types[i-1] == tSTART_LINK {
				expect(tLINK_LABEL)
				label = expect(tSTR)
				expect(tLINK)
			}
			link := texts[i-1]
			to := node()
			arrowTypeEnd := edgeStrToEdgeData(link)
			arrowTypeStart := edgeStrToEdgeStartData(link)
			thickness := edgeStrToThickness(link)
			pattern := edgeStrToPattern(link)
			// Mermaid keeps only the last link of `A --> B --> C` and loses its start. Here every link counts.
			if chain == nil {
				chain = []*Block{{ID: from.ID, Label: from.Label, Type: from.Type, Directions: from.Directions, WidthInColumns: 1, Columns: -1}}
			}
			prev := chain[len(chain)-1].ID
			from = &Block{ID: to.id, Label: to.label, Type: typeStr2Type(to.typeStr), Directions: to.directions, WidthInColumns: 1, Columns: -1}
			chain = append(chain,
				&Block{
					ID:             prev + "-" + to.id,
					Start:          prev,
					End:            to.id,
					Label:          label,
					Type:           "edge",
					Thickness:      thickness,
					Pattern:        pattern,
					Directions:     to.directions,
					ArrowTypeEnd:   arrowTypeEnd,
					ArrowTypeStart: arrowTypeStart,
					WidthInColumns: 1,
					Columns:        -1,
				},
				from)
		}
		if chain != nil {
			return statement{chain: chain}
		}
		return statement{block: from}
	}

	expect(tKEY)
	// Composites nest without recursion: each open one keeps the statement list of its parent.
	var open []frame
	var list []*Block
	for {
		switch types[i] {
		case tNODE_ID:
			list = appendStatement(list, nodeStatement())
		case tCOLUMNS:
			list = append(list, &Block{Type: "column-setting", Columns: js.ParseInt(texts[i], 10), WidthInColumns: 1})
			i++
		case tSPACE_BLOCK:
			width := js.ParseInt(texts[i], 10)
			i++
			list = append(list, &Block{ID: db.generateID(), Type: "space", Width: width, WidthInColumns: 1, Columns: -1})
		case tKEY:
			i++
			open = append(open, frame{list: list})
			list = nil
		case tID_BLOCK:
			i++
			if types[i] != tNODE_ID {
				fail("'NODE_ID'")
			}
			h := nodeStatement()
			open = append(open, frame{list: list, header: &h})
			list = nil
		case tEnd:
			if len(open) == 0 || len(list) == 0 {
				fail(statementStart)
			}
			f := open[len(open)-1]
			open = open[:len(open)-1]
			i++
			id := db.generateID()
			children := list
			list = f.list
			var b *Block
			switch {
			case f.header == nil:
				b = &Block{ID: id, Type: "composite", Children: children, WidthInColumns: 1, Columns: -1}
			case f.header.chain != nil:
				// Spreading the list of a link statement copies no block field.
				b = &Block{NoID: true, Type: "composite", Children: children, WidthInColumns: 1, Columns: -1}
			default:
				c := *f.header.block
				c.Type = "composite"
				c.Children = children
				b = &c
			}
			list = append(list, b)
		case tCLASSDEF:
			i++
			id := js.Trim(expect(tCLASSDEF_ID))
			list = append(list, &Block{Type: "classDef", ID: id, CSS: js.Trim(expect(tCLASSDEF_STYLEOPTS)), WidthInColumns: 1, Columns: -1})
		case tCLASS:
			i++
			id := js.Trim(expect(tCLASSENTITY_IDS))
			list = append(list, &Block{Type: "applyClass", ID: id, StyleClass: js.Trim(expect(tSTYLECLASS)), WidthInColumns: 1, Columns: -1})
		case tSTYLE:
			i++
			id := js.Trim(expect(tSTYLE_ENTITY_IDS))
			list = append(list, &Block{Type: "applyStyles", ID: id, StylesStr: js.Trim(expect(tSTYLE_DEFINITION_DATA)), WidthInColumns: 1, Columns: -1})
		case tEOF:
			if len(open) > 0 {
				fail(statementStart + ", 'end'")
			}
			if len(list) == 0 {
				fail(statementStart)
			}
			db.setHierarchy(list)
			return
		default:
			switch {
			case len(list) == 0:
				fail(statementStart)
			case len(open) > 0:
				fail(statementStart + ", 'end'")
			default:
				fail(statementStart + ", 'EOF'")
			}
		}
	}
}
