package gantt

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// builder takes the calls the grammar's actions make, under Mermaid's names.
type builder interface {
	setDateFormat(txt string)
	enableInclusiveEndDates()
	topAxis()
	setAxisFormat(txt string)
	setTickInterval(txt string)
	setExcludes(txt string)
	setIncludes(txt string)
	setTodayMarker(txt string)
	setWeekday(txt string)
	setWeekend(txt string)
	setDiagramTitle(txt string)
	setAccTitle(txt string)
	setAccDescription(txt string)
	addSection(txt string)
	addTask(descr, data string)
	// args is nil when the click names no arguments.
	setClickEvent(ids, functionName string, functionArgs *string)
	setLink(ids, link string)
}

var weekdays = []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

const line = "'EOF', 'SPACE', 'NL', 'weekday_monday', 'weekday_tuesday', 'weekday_wednesday', 'weekday_thursday', " +
	"'weekday_friday', 'weekday_saturday', 'weekday_sunday', 'weekend_friday', 'weekend_saturday', 'dateFormat', " +
	"'inclusiveEndDates', 'topAxis', 'axisFormat', 'tickInterval', 'excludes', 'includes', 'todayMarker', 'title', " +
	"'acc_title', 'acc_descr', 'acc_descr_multiline_value', 'section', 'taskTxt', 'click'"

// startsLine marks the tokens that can start a line, which are also the only ones that can
// follow a complete statement.
var startsLine = func() [tEOF + 1]bool {
	var s [tEOF + 1]bool
	for t := tWeekdayMonday; t <= tWeekendSaturday; t++ {
		s[t] = true
	}
	for _, t := range []int{tEOF, tNL, tDateFormat, tInclusiveEndDates, tTopAxis, tAxisFormat, tTickInterval, tExcludes,
		tIncludes, tTodayMarker, tTitle, tAccTitle, tAccDescr, tAccDescrMultilineValue, tSection, tTaskTxt, tClick} {
		s[t] = true
	}
	return s
}()

func parseGantt(src string, db builder) {
	tk := tokenize(src)
	types, texts := tk.types, tk.texts
	i := 0
	fail := func(expected string) {
		got := tokenNames[types[i]]
		if types[i] == tEnd {
			got = "1"
		}
		panic(core.SyntaxError("gantt", src, tk.starts[i], "Expecting "+expected+", got '"+got+"'", false))
	}
	// Mermaid's parser runs a statement's action only after seeing that the next token can follow it.
	end := func(also string) {
		if !startsLine[types[i]] {
			fail(line + also)
		}
	}
	if types[i] != tGantt {
		fail("'gantt'")
	}
	i++
	for {
		t := types[i]
		text := texts[i]
		if !startsLine[t] {
			fail(line)
		}
		i++
		// A statement's text starts after its keyword and the one blank that follows it.
		rest := func(n int) string { return js.SliceFrom(text, n) }
		switch t {
		case tEOF:
			if types[i] == tEnd {
				return
			}
			end("")
		case tNL:
		case tDateFormat:
			end("")
			db.setDateFormat(rest(11))
		case tInclusiveEndDates:
			end("")
			db.enableInclusiveEndDates()
		case tTopAxis:
			end("")
			db.topAxis()
		case tAxisFormat:
			end("")
			db.setAxisFormat(rest(11))
		case tTickInterval:
			end("")
			db.setTickInterval(rest(13))
		case tExcludes:
			end("")
			db.setExcludes(rest(9))
		case tIncludes:
			end("")
			db.setIncludes(rest(9))
		case tTodayMarker:
			end("")
			db.setTodayMarker(rest(12))
		case tWeekendFriday:
			end("")
			db.setWeekend("friday")
		case tWeekendSaturday:
			end("")
			db.setWeekend("saturday")
		case tTitle:
			end("")
			db.setDiagramTitle(rest(6))
		case tAccTitle:
			if types[i] != tAccTitleValue {
				fail("'acc_title_value'")
			}
			value := texts[i]
			i++
			end("")
			db.setAccTitle(js.Trim(value))
		case tAccDescr:
			if types[i] != tAccDescrValue {
				fail("'acc_descr_value'")
			}
			value := texts[i]
			i++
			end("")
			db.setAccDescription(js.Trim(value))
		case tAccDescrMultilineValue:
			end("")
			db.setAccDescription(js.Trim(text))
		case tSection:
			end("")
			db.addSection(rest(8))
		case tTaskTxt:
			if types[i] != tTaskData {
				fail("'taskData'")
			}
			data := texts[i]
			i++
			end("")
			db.addTask(text, data)
		case tClick:
			var name, href *string
			var args *string
			take := func() *string {
				s := texts[i]
				i++
				return &s
			}
			if types[i] == tCallbackname {
				name = take()
				if types[i] == tCallbackargs {
					args = take()
				}
				if types[i] == tHref {
					href = take()
					end("")
				} else if args == nil {
					end(", 'callbackargs', 'href'")
				} else {
					end(", 'href'")
				}
			} else if types[i] == tHref {
				href = take()
				if types[i] == tCallbackname {
					name = take()
					if types[i] == tCallbackargs {
						args = take()
						end("")
					} else {
						end(", 'callbackargs'")
					}
				} else {
					end(", 'callbackname'")
				}
			} else {
				fail("'callbackname', 'href'")
			}
			if name != nil {
				db.setClickEvent(text, *name, args)
			}
			if href != nil {
				db.setLink(text, *href)
			}
		default:
			end("")
			db.setWeekday(weekdays[t-tWeekdayMonday])
		}
	}
}
