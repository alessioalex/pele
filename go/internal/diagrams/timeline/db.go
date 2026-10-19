package timeline

import (
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

// Period is a time period and its events.
type Period struct {
	Text   string
	Events []string
	// Name of the section in force when the period was declared, "" before the first one.
	Section string
	// Index into Sections of that section, -1 before the first one.
	SectionIndex int
}

// Db is a parsed timeline.
type Db struct {
	Title     string
	AccTitle  string
	AccDescr  string
	Direction string
	Sections  []string
	Periods   []*Period

	currentSection string
}

func newDb() *Db { return &Db{Direction: "LR"} }

func (db *Db) setDiagramTitle(text string) { db.Title = text }

func (db *Db) setAccTitle(text string) { db.AccTitle = js.TrimStart(text) }

func (db *Db) setAccDescription(text string) { db.AccDescr = collapseIndents(text) }

func (db *Db) setDirection(direction string) { db.Direction = direction }

func (db *Db) addSection(text string) {
	db.currentSection = text
	db.Sections = append(db.Sections, text)
}

func (db *Db) addTask(period, event string) {
	p := &Period{Text: period, Events: []string{}, Section: db.currentSection, SectionIndex: len(db.Sections) - 1}
	if event != "" {
		p.Events = append(p.Events, event)
	}
	db.Periods = append(db.Periods, p)
}

func (db *Db) addEvent(event string) {
	if len(db.Periods) == 0 {
		// Mermaid fails here too, with a TypeError.
		err := core.NewError("An event needs a time period before it.", core.CodeSemantic)
		err.Type = "timeline"
		panic(err)
	}
	period := db.Periods[len(db.Periods)-1]
	period.Events = append(period.Events, event)
}

// collapseIndents is .replace(/\n\s+/g, '\n').
func collapseIndents(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
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
