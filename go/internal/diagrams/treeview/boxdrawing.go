package treeview

import (
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Mermaid's box-drawing preprocessor: turns `├──`, `└──`, `│` (and the heavy variants) into the
// indentation the grammar reads.

const indentUnit = "    "

func isBoxChar(r rune) bool {
	switch r {
	case '─', '━', '│', '┃', '└', '┗', '├', '┣':
		return true
	}
	return false
}

func isBranchChar(r rune) bool { return r == '└' || r == '┗' || r == '├' || r == '┣' }

func isDashChar(r rune) bool { return r == '─' || r == '━' }

func hasBoxChar(s string) bool { return strings.IndexFunc(s, isBoxChar) >= 0 }

// allOf is /^[...]+$/ for a class given as a test.
func allOf(s string, in func(rune) bool) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !in(r) {
			return false
		}
	}
	return true
}

func decorationOnly(s string) bool {
	return allOf(s, func(r rune) bool { return js.IsSpace(r) || r == '│' || r == '┃' })
}

func boxOnly(s string) bool {
	return allOf(s, func(r rune) bool { return js.IsSpace(r) || isBoxChar(r) })
}

func skipSpace(s string) int {
	i := 0
	for i < len(s) {
		r, size := js.RuneAt(s, i)
		if !js.IsSpace(r) {
			break
		}
		i += size
	}
	return i
}

// metadataLine is /^\s*(title[\t ]|accTitle[\t ]*:|accDescr[\t ]*[:{])/.
func metadataLine(line string) bool {
	rest := line[skipSpace(line):]
	if strings.HasPrefix(rest, "title") {
		return len(rest) > 5 && isBlank(rest[5])
	}
	if strings.HasPrefix(rest, "accTitle") {
		return byteAt(rest, skipBlanks(rest, 8)) == ':'
	}
	if strings.HasPrefix(rest, "accDescr") {
		c := byteAt(rest, skipBlanks(rest, 8))
		return c == ':' || c == '{'
	}
	return false
}

// commentLine is /^\s*%%/.
func commentLine(line string) bool { return strings.HasPrefix(line[skipSpace(line):], "%%") }

func untab(s string) string { return strings.ReplaceAll(s, "\t", "    ") }

// lineMap maps output line numbers to original ones, both from 1.
type lineMap map[int]int

func inferSegmentWidth(contentLines []string) int {
	for _, line := range contentLines {
		if at := strings.IndexFunc(line, isBranchChar); at > 0 {
			return js.UnitsBefore(line, at)
		}
	}
	return 4
}

// remapErrorLines is message.replace(/\bline\s+(\d+)\b/gi, ...), which writes the word in lower case.
func remapErrorLines(message string, m lineMap) string {
	var b strings.Builder
	last := 0
	for i := 0; i+4 <= len(message); i++ {
		if !js.HasPrefixFold(message[i:], "line") {
			continue
		}
		if i > 0 && isWordByte(message[i-1]) {
			continue
		}
		j := i + 4
		k := j
		for k < len(message) {
			r, size := js.RuneAt(message, k)
			if !js.IsSpace(r) {
				break
			}
			k += size
		}
		if k == j {
			continue
		}
		d := k
		for d < len(message) && message[d] >= '0' && message[d] <= '9' {
			d++
		}
		// \d+ backtracks, but a shorter run ends between two digits, which is no boundary.
		if d == k || (d < len(message) && isWordByte(message[d])) {
			continue
		}
		n, _ := strconv.Atoi(message[k:d])
		original, ok := m[n]
		if len(message[k:d]) > 15 {
			ok = false
		}
		b.WriteString(message[last:i])
		if ok && original != 0 {
			b.WriteString("line " + strconv.Itoa(original))
		} else {
			b.WriteString(message[i:d])
		}
		last = d
		i = d - 1
	}
	b.WriteString(message[last:])
	return b.String()
}

func boxFail(line int, message string) {
	panic(&core.Error{Message: "Line " + strconv.Itoa(line) + ": " + message, Code: core.CodeSyntax, Type: "treeView", Line: line})
}

func preprocessBoxDrawing(input string) (string, lineMap) {
	m := lineMap{}
	if !hasBoxChar(input) {
		return input, m
	}
	lines := strings.Split(input, "\n")

	keywordIdx := -1
	for i, line := range lines {
		if js.Trim(line) == "treeView-beta" {
			keywordIdx = i
			break
		}
	}
	if keywordIdx == -1 {
		return input, m
	}

	passes := func(line string) bool { return js.Trim(line) == "" || commentLine(line) || metadataLine(line) }

	var contentLines []string
	for _, line := range lines[keywordIdx+1:] {
		if passes(line) || decorationOnly(line) {
			continue
		}
		contentLines = append(contentLines, untab(line))
	}
	found := false
	for _, line := range contentLines {
		if hasBoxChar(line) {
			found = true
			break
		}
	}
	if !found {
		return input, m
	}

	segmentWidth := inferSegmentWidth(contentLines)
	var out []string
	keep := func(text string, index int) {
		out = append(out, text)
		m[len(out)] = index + 1
	}

	for i := 0; i <= keywordIdx; i++ {
		keep(lines[i], i)
	}
	for i := keywordIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if passes(line) {
			keep(line, i)
			continue
		}
		if decorationOnly(line) {
			continue
		}

		normalized := untab(line)
		if at := strings.IndexFunc(normalized, isBranchChar); at >= 0 {
			depth := int(js.Round(float64(js.UnitsBefore(normalized, at))/float64(segmentWidth))) + 1
			_, size := js.RuneAt(normalized, at)
			pos := at + size
			for pos < len(normalized) {
				r, size := js.RuneAt(normalized, pos)
				if !isDashChar(r) {
					break
				}
				pos += size
			}
			for pos < len(normalized) && normalized[pos] == ' ' {
				pos++
			}
			content := js.TrimEnd(normalized[pos:])
			if content == "" {
				boxFail(i+1, "Empty node — expected a filename or directory name after the box-drawing prefix")
			}
			keep(strings.Repeat(indentUnit, depth)+content, i)
		} else if boxOnly(normalized) {
			continue
		} else if hasBoxChar(normalized) {
			// A name that happens to contain a box character, such as "Section ─ A.txt", is a root item.
			keep(line, i)
		} else if skipSpace(normalized) > 0 {
			boxFail(i+1, "Unexpected indentation without box-drawing characters. In box-drawing format, use ├── or └── prefixes for indented nodes.")
		} else {
			keep(line, i)
		}
	}
	return strings.Join(out, "\n"), m
}
