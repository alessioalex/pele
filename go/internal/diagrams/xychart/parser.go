package xychart

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

func flags(tokens ...int) []bool {
	out := make([]bool, tEOF+1)
	for _, t := range tokens {
		out[t] = true
	}
	return out
}

var (
	wordTokens = flags(tAmp, tNum, tAlpha, tPlus, tEquals, tMult, tDot, tBrkt, tMinus, tUnderscore)
	eolTokens  = flags(tNEWLINE, tSemi, tEOF)
)

func parseXyChart(src string, db *Db) {
	toks := tokenize(src)
	types, texts, starts := toks.types, toks.texts, toks.starts
	i := 0

	kind := func(k int) int {
		if k < len(types) {
			return types[k]
		}
		return -1
	}
	isWordToken := func(t int) bool { return t >= 0 && wordTokens[t] }
	isEOL := func(t int) bool { return t >= 0 && eolTokens[t] }

	fail := func(expected string) {
		t := types[i]
		if t == tERROR {
			panic(core.SyntaxError("xychart", src, starts[i], "", true))
		}
		panic(core.SyntaxError("xychart", src, starts[i], "Expecting "+expected+", got '"+tokenNames[t]+"'", false))
	}

	expect := func(typ int) string {
		if types[i] != typ {
			fail("'" + tokenNames[typ] + "'")
		}
		i++
		return texts[i-1]
	}

	isText := func(t int) bool { return t == tStr || isWordToken(t) }

	readText := func() string {
		if !isText(types[i]) {
			fail("'STR', 'ALPHA', 'NUM'")
		}
		first := types[i]
		s := texts[i]
		i++
		if first != tStr && isWordToken(kind(i)) {
			var b strings.Builder
			b.WriteString(s)
			for isWordToken(kind(i)) {
				b.WriteString(texts[i])
				i++
			}
			s = b.String()
		}
		return s
	}

	rangeData := func() (float64, float64) {
		min := js.ToNumber(expect(tNumberWithDecimal))
		expect(tArrowDelimiter)
		return min, js.ToNumber(expect(tNumberWithDecimal))
	}

	points := func() []dataPoint {
		var out []dataPoint
		expect(tSquareBracesStart)
		for {
			value := js.ToNumber(expect(tNumberWithDecimal))
			label := ""
			if types[i] == tStr {
				label = texts[i]
				i++
			}
			out = append(out, dataPoint{value: value, label: label})
			if types[i] != tComma {
				break
			}
			i++
		}
		expect(tSquareBracesEnd)
		return out
	}

	for {
		if isEOL(types[i]) {
			i++
		} else if types[i] != tXYCHART {
			break
		} else {
			i++
			if types[i] == tChartOrientation {
				db.setOrientation(texts[i])
				i++
			}
		}
	}

	for types[i] != tEND {
		t := types[i]
		i++
		switch t {
		case tTitle:
			db.Title = js.Trim(readText())
		case tXAxis, tYAxis:
			title, hasTitle := "", false
			if isText(types[i]) {
				title, hasTitle = readText(), true
			}
			if t == tXAxis && types[i] == tSquareBracesStart {
				var categories []string
				for {
					i++
					categories = append(categories, readText())
					if types[i] != tComma {
						break
					}
				}
				expect(tSquareBracesEnd)
				db.setXAxisBand(categories)
			} else if types[i] == tNumberWithDecimal || !hasTitle {
				min, max := rangeData()
				if t == tXAxis {
					db.setXAxisRangeData(min, max)
				} else {
					db.setYAxisRangeData(min, max)
				}
			}
			if t == tXAxis {
				db.XAxis.Title = js.Trim(title)
			} else {
				db.YAxis.Title = js.Trim(title)
			}
		case tLine, tBar:
			title := ""
			if types[i] != tSquareBracesStart {
				title = readText()
			}
			data := points()
			if t == tLine {
				db.addPlot("line", title, data)
			} else {
				db.addPlot("bar", title, data)
			}
		case tAccTitle:
			db.setAccTitle(js.Trim(expect(tAccTitleValue)))
		case tAccDescr:
			db.setAccDescription(js.Trim(expect(tAccDescrValue)))
		case tAccDescrMultilineValue:
			db.setAccDescription(js.Trim(texts[i-1]))
		default:
			i--
			fail("'title', 'X_AXIS', 'Y_AXIS', 'LINE', 'BAR', 'acc_title', 'acc_descr', 'acc_descr_multiline_value'")
		}
		for isEOL(kind(i)) {
			i++
		}
	}
}
