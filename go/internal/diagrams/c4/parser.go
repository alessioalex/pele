package c4

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Mermaid's typeC4Shape for each shape keyword, indexed from tPERSON.
var kinds = []string{
	"person",
	"external_person",
	"system",
	"system_db",
	"system_queue",
	"external_system",
	"external_system_db",
	"external_system_queue",
	"container",
	"container_db",
	"container_queue",
	"external_container",
	"external_container_db",
	"external_container_queue",
	"component",
	"component_db",
	"component_queue",
	"external_component",
	"external_component_db",
	"external_component_queue",
}

var relTypes = []string{"rel", "birel", "rel_u", "rel_d", "rel_l", "rel_r", "rel_b", "rel"}
var nodeTypes = []string{"node", "nodeL", "nodeR"}

const headers = "'direction_tb', 'direction_bt', 'direction_rl', 'direction_lr', 'C4_CONTEXT', 'C4_CONTAINER', 'C4_COMPONENT', 'C4_DYNAMIC', 'C4_DEPLOYMENT'"
const attributeNames = "'STR', 'STR_KEY', 'ATTRIBUTE', 'ATTRIBUTE_EMPTY'"

const (
	// A statement is required.
	pItem = iota
	// Only a diagram statement may come next: the first one in a boundary, or one written straight after a title.
	pDiagram
	pItemOrEnd
	pAfterOther
	pAfterDiagram
)

func isOther(t int) bool {
	return t == tTitle || t == tAccDescription || t == tAccTitle || t == tAccDescr || t == tAccDescrMultilineValue
}

func isDiagram(t int) bool {
	return t >= tPERSON && t <= tUPDATE_LAYOUT_CONFIG
}

// insert is attrs.splice(at, 0, value), which inserts at the end when at is past it.
func insert(attrs []Attr, at int, value string) []Attr {
	if at > len(attrs) {
		at = len(attrs)
	}
	out := make([]Attr, 0, len(attrs)+1)
	out = append(out, attrs[:at]...)
	out = append(out, Attr{Str: value})
	return append(out, attrs[at:]...)
}

// dropWord is .substring(len(word) + 1): the word and the one whitespace character after it.
func dropWord(s string, word int) string {
	_, size := spaceAt(s, word)
	if size == 0 {
		size = 1
	}
	if word+size > len(s) {
		return ""
	}
	return s[word+size:]
}

// Accepts what the parser Jison generates from c4Diagram.jison accepts, and calls the model
// builder in the order its semantic actions do. Boundaries nest through a counter, not recursion.
func parseC4(src string, db *Db) {
	tk := tokenize(src)
	types, starts, ends := tk.types, tk.starts, tk.ends
	i := 0

	kind := func(at int) int {
		if at >= len(types) {
			return tEND
		}
		return types[at]
	}
	text := func(at int) string { return src[starts[at]:ends[at]] }
	fail := func(expected string) {
		t := kind(i)
		at := len(src)
		if i < len(starts) {
			at = starts[i]
		}
		if t == tERROR {
			panic(core.SyntaxError("c4", src, at, "", true))
		}
		panic(core.SyntaxError("c4", src, at, "Expecting "+expected+", got '"+tokenNames[t]+"'", false))
	}
	expect := func(typ int) {
		if kind(i) != typ {
			fail("'" + tokenNames[typ] + "'")
		}
		i++
	}

	attributes := func() []Attr {
		var attrs []Attr
	read:
		for {
			switch kind(i) {
			case tSTR:
				attrs = append(attrs, Attr{Str: js.Trim(text(i))})
				i++
			case tATTRIBUTE_EMPTY:
				attrs = append(attrs, Attr{})
				i++
			case tSTR_KEY:
				i++
				if kind(i) != tSTR_VALUE {
					fail("'STR_VALUE'")
				}
				attrs = append(attrs, Attr{Named: &Named{Key: js.Trim(text(i - 1)), Value: js.Trim(text(i))}})
				i++
			default:
				break read
			}
		}
		if len(attrs) == 0 {
			fail(attributeNames)
		}
		return attrs
	}

	other := func(t int) {
		switch t {
		case tTitle:
			db.setTitle(dropWord(text(i), 5))
			i++
		case tAccDescription:
			db.setAccDescription(dropWord(text(i), 14))
			i++
		case tAccDescrMultilineValue:
			db.setAccDescription(js.Trim(text(i)))
			i++
		default:
			i++
			if t == tAccTitle {
				if kind(i) != tAccTitleValue {
					fail("'acc_title_value'")
				}
				// The grammar hands `accTitle:` to setTitle, so it becomes the visible title.
				db.setTitle(js.Trim(text(i)))
				i++
			} else {
				if kind(i) != tAccDescrValue {
					fail("'acc_descr_value'")
				}
				db.setAccDescription(js.Trim(text(i)))
				i++
			}
		}
	}

	// Returns true when the statement opens a boundary.
	diagram := func(t int) bool {
		i++
		attrs := attributes()
		switch {
		case t <= tSYSTEM_EXT_QUEUE:
			db.addPersonOrSystem(kinds[t-tPERSON], attrs)
		case t <= tCONTAINER_EXT_QUEUE:
			db.addContainer(kinds[t-tPERSON], attrs)
		case t <= tCOMPONENT_EXT_QUEUE:
			db.addComponent(kinds[t-tPERSON], attrs)
		case t <= tNODE_R:
			switch t {
			case tENTERPRISE_BOUNDARY:
				attrs = insert(attrs, 2, "ENTERPRISE")
			case tSYSTEM_BOUNDARY:
				attrs = insert(attrs, 2, "SYSTEM")
			case tCONTAINER_BOUNDARY:
				attrs = insert(attrs, 2, "CONTAINER")
			}
			if t >= tNODE {
				db.addDeploymentNode(nodeTypes[t-tNODE], attrs)
			} else if t == tCONTAINER_BOUNDARY {
				db.addContainerBoundary(attrs)
			} else {
				db.addPersonOrSystemBoundary(attrs, "system")
			}
			switch kind(i) {
			case tLBRACE:
				i++
				expect(tNEWLINE)
			case tNEWLINE:
				i++
				expect(tLBRACE)
				if kind(i) == tNEWLINE {
					i++
				}
			default:
				fail("'NEWLINE', 'LBRACE'")
			}
			return true
		case t <= tREL_INDEX:
			// RelIndex takes a sequence number first, which Mermaid ignores.
			if t == tREL_INDEX {
				attrs = attrs[1:]
			}
			db.addRel(relTypes[t-tREL], attrs)
		case t == tUPDATE_EL_STYLE:
			db.updateElStyle(attrs)
		case t == tUPDATE_REL_STYLE:
			db.updateRelStyle(attrs)
		default:
			db.updateLayoutConfig(attrs)
		}
		return false
	}

	header := kind(i)
	if header < tC4Context || header > tC4Deployment {
		fail(headers)
	}
	i++
	expect(tNEWLINE)

	depth := 0
	state := pItem
	for {
		t := kind(i)
		if state == pAfterOther || state == pAfterDiagram {
			if t == tNEWLINE {
				i++
				state = pItemOrEnd
				continue
			}
			if state == pAfterOther && isDiagram(t) {
				state = pDiagram
				continue
			}
		} else if isDiagram(t) {
			if diagram(t) {
				depth++
				state = pDiagram
			} else {
				state = pAfterDiagram
			}
			continue
		} else if isOther(t) && state != pDiagram {
			other(t)
			state = pAfterOther
			continue
		} else if state != pItemOrEnd {
			fail("'title', 'accDescription', 'acc_title', 'acc_descr', 'acc_descr_multiline_value', 'PERSON', 'REL'")
		}

		if depth > 0 {
			if t != tRBRACE {
				fail("'NEWLINE', 'RBRACE'")
			}
			i++
			db.popBoundaryParseStack()
			depth--
			state = pAfterDiagram
		} else {
			if t != tEOF {
				fail("'NEWLINE', 'EOF'")
			}
			i++
			expect(tEND)
			db.setC4Type(text(0))
			return
		}
	}
}
