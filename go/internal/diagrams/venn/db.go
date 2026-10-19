package venn

import (
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// normalizeText takes one pair of quotes off an id, label or style value, as Mermaid does.
func normalizeText(text string) string {
	trimmed := js.Trim(text)
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		return trimmed[1 : len(trimmed)-1]
	}
	return trimmed
}

// compareUnits compares strings by UTF-16 code units, as Array.prototype.sort does.
func compareUnits(a, b string) int {
	for a != "" && b != "" {
		ra, sa := js.RuneAt(a, 0)
		rb, sb := js.RuneAt(b, 0)
		if ra != rb {
			ua, ub := unitsOf(ra), unitsOf(rb)
			if ua[0] != ub[0] {
				return int(ua[0]) - int(ub[0])
			}
			return int(ua[1]) - int(ub[1])
		}
		a, b = a[sa:], b[sb:]
	}
	return len(a) - len(b)
}

func unitsOf(r rune) [2]uint16 {
	if r >= 0x10000 {
		hi, lo := utf16.EncodeRune(r)
		return [2]uint16{uint16(hi), uint16(lo)}
	}
	return [2]uint16{uint16(r), 0}
}

func sortUnits(list []string) {
	slices.SortStableFunc(list, compareUnits)
}

func normalizeIds(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = normalizeText(id)
	}
	sortUnits(out)
	return out
}

// DB is a parsed venn diagram.
type DB struct {
	Title     string
	Subsets   []Subset
	TextNodes []Text
	Styles    []Style
	known     map[string]bool
	current   []string
	// Whether an indented `text` line belongs to the set or union before it. The lexer reads this.
	indent bool
}

func newDB() *DB { return &DB{known: map[string]bool{}} }

func optionalLabel(label *string) *string {
	if label == nil || *label == "" {
		return nil
	}
	l := normalizeText(*label)
	return &l
}

// addSubsetData gives a set without a size 10, and an overlap of n sets 10 / n².
func (db *DB) addSubsetData(ids []string, label *string, size *float64) {
	sets := normalizeIds(ids)
	db.current = sets
	if len(sets) == 1 {
		db.known[sets[0]] = true
	}
	s := 0.0
	if size != nil {
		s = *size
	} else {
		s = 10 / js.Pow(float64(len(ids)), 2)
	}
	db.Subsets = append(db.Subsets, Subset{Sets: sets, Size: s, Label: optionalLabel(label)})
}

func (db *DB) addTextData(ids []string, id string, label *string) {
	db.TextNodes = append(db.TextNodes, Text{Sets: normalizeIds(ids), ID: normalizeText(id), Label: optionalLabel(label)})
}

func (db *DB) addStyleData(ids []string, data [][2]string) {
	var styles orderedMap
	for _, kv := range data {
		value := kv[1]
		if value != "" {
			value = normalizeText(value)
		}
		styles.set(kv[0], value)
	}
	db.Styles = append(db.Styles, Style{Targets: normalizeIds(ids), Styles: styles})
}

func (db *DB) validateUnionIdentifiers(ids []string) {
	var unknown []string
	for _, id := range ids {
		if n := normalizeText(id); !db.known[n] {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		panic(core.NewError("unknown set identifier: "+strings.Join(unknown, ", "), core.CodeSemantic))
	}
}
