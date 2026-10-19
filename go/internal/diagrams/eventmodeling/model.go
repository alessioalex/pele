package eventmodeling

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

// EntityKind is ui, processor, command, readmodel or event.
type EntityKind string

// LaneGroup is trigger, state or stream.
type LaneGroup string

type EmLane struct {
	// Mermaid's ordering key: 0 for UI/Automation, 100 for Command/Read Model, 200 for Events,
	// and the next free number after those for each namespace.
	Index int
	Label string
	Group LaneGroup
	// "" with HasNamespace false for none.
	Namespace    string
	HasNamespace bool
}

type EmBox struct {
	// The frame number as written, such as "01".
	ID        string
	Kind      EntityKind
	Entity    string
	Name      string
	Namespace string
	Reset     bool
	Lane      *EmLane
	// Example data from the frame or from the data block it refers to; "" for none.
	Data     string
	DataType string
}

type EmRelation struct {
	Source, Target *EmBox
}

type EmNote struct {
	Frame, Text, DataType string
}

type EmSpecification struct {
	Frame             string
	Given, When, Then []string
}

// Model is a parsed event model.
type Model struct {
	Title, AccTitle, AccDescr string
	Boxes                     []*EmBox
	// Sorted from top to bottom.
	Lanes          []*EmLane
	Relations      []EmRelation
	Notes          []EmNote
	Specifications []EmSpecification
	Entities       []string
}

var kindsByName = map[string]EntityKind{
	"ui":        "ui",
	"pcr":       "processor",
	"processor": "processor",
	"rmo":       "readmodel",
	"readmodel": "readmodel",
	"cmd":       "command",
	"command":   "command",
	"evt":       "event",
	"event":     "event",
}

type groupInfo struct {
	first         int
	label, prefix string
}

var groups = map[LaneGroup]groupInfo{
	"trigger": {0, "UI/Automation", "UI/A: "},
	"state":   {100, "Command/Read Model", "C/RM: "},
	"stream":  {200, "Events", "Stream: "},
}

func groupOf(kind EntityKind) LaneGroup {
	if kind == "ui" || kind == "processor" {
		return "trigger"
	}
	if kind == "event" {
		return "stream"
	}
	return "state"
}

// blockText is the text between the outer braces, without the indentation its lines share.
func blockText(raw string) string {
	open := strings.IndexByte(raw, '{')
	close := strings.LastIndexByte(raw, '}')
	inner := ""
	if open != -1 && close > open {
		inner = raw[open+1 : close]
	} else if open == -1 && js.Len(raw) >= 2 {
		// The quotes of an inline value.
		inner = raw[1 : len(raw)-1]
	}
	lines := strings.Split(strings.ReplaceAll(inner, "\r", ""), "\n")
	for len(lines) > 0 && js.Trim(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && js.Trim(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	indent := math.MaxInt
	for _, line := range lines {
		if js.Trim(line) == "" {
			continue
		}
		indent = min(indent, js.Len(line)-js.Len(js.TrimStart(line)))
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = js.TrimEnd(js.SliceFrom(line, min(indent, js.Len(line))))
	}
	return strings.Join(out, "\n")
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

func buildEventModel(a *eventModelAst, title string) *Model {
	ids := map[string]bool{}
	for _, frame := range a.frames {
		if ids[frame.name] {
			line := core.Locate(a.source, frame.offset).Line
			panic(&core.Error{
				Message: `Duplicate event modeling frame ID "` + frame.name + `" on line ` + strconv.Itoa(line),
				Code:    core.CodeSemantic,
				Type:    "eventmodeling",
				Line:    line,
			})
		}
		ids[frame.name] = true
	}

	data := map[string]string{}
	for _, entity := range a.dataEntities {
		if _, ok := data[entity.name]; !ok {
			data[entity.name] = entity.dataBlockValue
		}
	}

	// One lane per group, and one more per namespace within a group.
	laneOf := map[string]*EmLane{}
	var laneOrder []*EmLane
	next := map[LaneGroup]int{}
	lane := func(kind EntityKind, namespace string, hasNamespace bool) *EmLane {
		group := groupOf(kind)
		key := string(group)
		if hasNamespace {
			key += " " + namespace
		}
		found := laneOf[key]
		if found == nil {
			g := groups[group]
			index := g.first
			if hasNamespace {
				if n, ok := next[group]; ok {
					index = n
				} else {
					index = g.first + 1
				}
				next[group] = index + 1
			}
			label := g.label
			if hasNamespace {
				label = g.prefix + namespace
			}
			found = &EmLane{Index: index, Label: label, Group: group, Namespace: namespace, HasNamespace: hasNamespace}
			laneOf[key] = found
			laneOrder = append(laneOrder, found)
		}
		return found
	}

	m := &Model{}
	var relations []EmRelation
	for index, frame := range a.frames {
		// Only a two-part name has a namespace.
		parts := strings.Split(frame.entityIdentifier, ".")
		namespace := ""
		if len(parts) == 2 {
			namespace = parts[0]
		}
		kind, ok := kindsByName[frame.modelEntityType]
		if !ok {
			kind = "event"
		}
		text := ""
		if frame.dataInlineValue != "" {
			text = blockText(frame.dataInlineValue)
		}
		if frame.dataReference != "" {
			if block, ok := data[frame.dataReference]; ok {
				text = blockText(block)
			}
		}
		name := frame.entityIdentifier
		if len(parts) == 2 {
			name = parts[1]
		}
		box := &EmBox{
			ID:        frame.name,
			Kind:      kind,
			Entity:    frame.entityIdentifier,
			Name:      name,
			Namespace: namespace,
			Reset:     frame.reset,
			Lane:      lane(kind, namespace, len(parts) == 2),
			Data:      text,
			DataType:  frame.dataType,
		}
		m.Boxes = append(m.Boxes, box)

		if len(frame.sourceFrames) > 0 {
			// Sources connect in frame order, and only frames that already have a box.
			wanted := map[string]bool{}
			for _, ref := range frame.sourceFrames {
				wanted[ref] = true
			}
			for _, source := range m.Boxes {
				if wanted[source.ID] {
					relations = append(relations, EmRelation{source, box})
				}
			}
		} else if index > 0 && !box.Reset {
			// Otherwise the nearest earlier frame in another lane feeds this one.
			for i := index - 1; i >= 0; i-- {
				if m.Boxes[i].Lane != box.Lane {
					relations = append(relations, EmRelation{m.Boxes[i], box})
					break
				}
			}
		}
	}

	if a.title != "" {
		m.Title = common.TitleValue(a.title)
	}
	if m.Title == "" {
		m.Title = title
	}
	if a.accTitle != "" {
		m.AccTitle = js.TrimStart(common.AccTitleValue(a.accTitle))
	}
	if a.accDescr != "" {
		m.AccDescr = collapseIndents(common.AccDescrValue(a.accDescr))
	}
	m.Lanes = laneOrder
	slices.SortStableFunc(m.Lanes, func(x, y *EmLane) int { return x.Index - y.Index })
	m.Relations = relations
	for _, note := range a.noteEntities {
		m.Notes = append(m.Notes, EmNote{Frame: note.sourceFrame, Text: blockText(note.dataBlockValue), DataType: note.dataType})
	}
	for _, gwt := range a.gwtEntities {
		m.Specifications = append(m.Specifications, EmSpecification{Frame: gwt.sourceFrame, Given: gwt.givenStatements, When: gwt.whenStatements, Then: gwt.thenStatements})
	}
	m.Entities = a.modelEntities
	return m
}
