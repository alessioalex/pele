//go:build wip

package sequence

import (
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

func fail(message string) {
	e := core.NewError(message, core.CodeSemantic)
	e.Type = "sequence"
	panic(e)
}

// hasPrefixWrap reports the length of a /^:?wrap:/ or /^:?nowrap:/ prefix, or -1.
func wrapPrefix(text, word string) int {
	q := 0
	if strings.HasPrefix(text, ":") {
		q = 1
	}
	if strings.HasPrefix(text[q:], word+":") {
		return q + len(word) + 1
	}
	return -1
}

func extractWrap(raw string) *seqText {
	text := js.Trim(raw)
	if n := wrapPrefix(text, "wrap"); n >= 0 {
		return &seqText{text: js.Trim(text[n:]), wrap: triTrue}
	}
	if n := wrapPrefix(text, "nowrap"); n >= 0 {
		return &seqText{text: js.Trim(text[n:]), wrap: triFalse}
	}
	return &seqText{text: js.Trim(text), wrap: triUndefined}
}

func isBoxLineTerminator(r rune) bool { return r == '\r' || r == 0x2028 || r == 0x2029 }

func hasBoxLineTerminator(s string) bool { return strings.IndexFunc(s, isBoxLineTerminator) >= 0 }

// boxFunction matches /^(?:rgba?|hsla?)\s*\(/ and returns the length of the match, or -1.
func boxFunction(s string) int {
	q := 0
	switch {
	case strings.HasPrefix(s, "rgba"), strings.HasPrefix(s, "hsla"):
		q = 4
	case strings.HasPrefix(s, "rgb"), strings.HasPrefix(s, "hsl"):
		q = 3
	default:
		return -1
	}
	for q < len(s) {
		r, size := js.RuneAt(s, q)
		if !js.IsSpace(r) {
			break
		}
		q += size
	}
	if q < len(s) && s[q] == '(' {
		return q + 1
	}
	return -1
}

// splitBox splits a box line into a leading color and the rest, as Mermaid's
// /^((?:rgba?|hsla?)\s*\(.*\)|\w*)(.*)$/ does. `.` stops at a line terminator, so a line that
// holds one outside the gap before the parenthesis does not match at all.
func splitBox(str string) (string, string, bool) {
	if fn := boxFunction(str); fn >= 0 {
		end := strings.LastIndexByte(str, ')')
		if end >= fn {
			if hasBoxLineTerminator(str[fn:]) {
				return "", "", false
			}
			return str[:end+1], str[end+1:], true
		}
	}
	if hasBoxLineTerminator(str) {
		return "", "", false
	}
	w := 0
	for w < len(str) && js.IsWord(rune(str[w])) {
		w++
	}
	return str[:w], str[w:], true
}

// entries is Object.entries of a parsed JSON value, with a string read as its characters.
func entries(value core.Value) ([]string, []core.Value) {
	var keys []string
	var vals []core.Value
	switch x := value.(type) {
	case string:
		i := 0
		for _, r := range x {
			keys = append(keys, strconv.Itoa(i))
			vals = append(vals, string(r))
			i++
		}
	case []core.Value:
		for i, v := range x {
			keys = append(keys, strconv.Itoa(i))
			vals = append(vals, v)
		}
	case *core.Object:
		for _, k := range x.Keys() {
			keys = append(keys, k)
			vals = append(vals, x.Value(k))
		}
	}
	return keys, vals
}

type seqDb struct {
	actors          map[string]*seqActor
	actorOrder      []string
	createdActors   map[string]int
	destroyedActors map[string]int
	boxes           []*seqBox
	messages        []*seqMessage
	notes           []*seqNote
	title           string
	accTitle        string
	accDescr        string
	prevActor       string
	currentBox      *seqBox
	lastCreated     string
	lastDestroyed   string
	wrapEnabled     tri
	sequenceWrap    bool
	active          map[string]int
}

func newSeqDb(wrap tri, sequenceWrap bool) *seqDb {
	return &seqDb{
		actors:          map[string]*seqActor{},
		createdActors:   map[string]int{},
		destroyedActors: map[string]int{},
		active:          map[string]int{},
		wrapEnabled:     wrap,
		sequenceWrap:    sequenceWrap,
	}
}

func (db *seqDb) autoWrap() bool { return db.wrapEnabled.or(db.sequenceWrap) }

func (db *seqDb) setAccTitle(text string) { db.accTitle = js.TrimStart(text) }

// setAccDescription is .replace(/\n\s+/g, '\n').
func (db *seqDb) setAccDescription(s string) {
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
	db.accDescr = b.String()
}

func (db *seqDb) setDiagramTitle(text string) { db.title = text }

func (db *seqDb) parseMessage(str string) *seqText { return extractWrap(str) }

func (db *seqDb) parseBoxData(str string) *seqBoxData {
	first, rest, ok := splitBox(str)
	color := "transparent"
	if ok && first != "" {
		color = js.Trim(first)
	}
	title, hasTitle := "", false
	if ok && rest != "" {
		title, hasTitle = js.Trim(rest), true
	}
	if !isCSSColor(color) {
		color = "transparent"
		title, hasTitle = js.Trim(str), true
	}
	if !hasTitle {
		return &seqBoxData{color: color}
	}
	t := extractWrap(title)
	return &seqBoxData{text: t.text, color: color, wrap: t.wrap}
}

func (db *seqDb) addBox(data *seqBoxData) {
	db.currentBox = &seqBox{name: data.text, wrap: data.wrap.or(db.autoWrap()), fill: data.color}
	db.boxes = append(db.boxes, db.currentBox)
}

// boxName is `${box.name}`, where an untitled box's name is undefined.
func boxName(b *seqBox) string {
	if b.name == "" {
		return "undefined"
	}
	return b.name
}

// addActor takes the participant's kind in typ, "" when it has none.
func (db *seqDb) addActor(id, name string, description *seqText, typ string, metadata string, hasMetadata bool) {
	hasType := typ != ""
	assignedBox := db.currentBox
	var doc *core.Object
	if hasMetadata {
		src := "{\n" + metadata + "\n}"
		if strings.Contains(metadata, "\n") {
			src = metadata + "\n"
		}
		parsed, err := core.ParseYaml(src)
		if err != nil {
			e := core.NewError(`Config of participant "`+id+`" is not valid YAML. `+err.Error(), core.CodeSyntax)
			e.Type = "sequence"
			panic(e)
		}
		doc = core.Obj(parsed)
	}
	if doc != nil {
		if docType, ok := doc.Get("type"); ok && docType != nil {
			typ, hasType = core.ToString(docType), true
		}
		if docAlias, ok := doc.Get("alias"); ok && core.Truthy(docAlias) && (description == nil || description.text == name) {
			wrap := triUndefined
			if description != nil {
				wrap = description.wrap
			}
			description = &seqText{text: core.ToString(docAlias), wrap: wrap}
		}
	}

	if old := db.actors[id]; old != nil {
		if db.currentBox != nil && old.box != nil && db.currentBox != old.box {
			fail("A same participant should only be defined in one Box: " + old.name + " can't be in '" + boxName(old.box) +
				"' and in '" + boxName(db.currentBox) + "' at the same time.")
		}
		if old.box != nil {
			assignedBox = old.box
		} else {
			assignedBox = db.currentBox
		}
		old.box = assignedBox
		if name == old.name && description == nil {
			return
		}
	}
	// A participant that is only mentioned in a message has no kind and takes its name as its label.
	if description == nil || !hasType {
		description = &seqText{text: name}
	}

	if _, ok := db.actors[id]; !ok {
		db.actorOrder = append(db.actorOrder, id)
	}
	if !hasType {
		typ = "participant"
	}
	db.actors[id] = &seqActor{
		box:         assignedBox,
		name:        name,
		description: description.text,
		wrap:        description.wrap.or(db.autoWrap()),
		prevActor:   db.prevActor,
		links:       map[string]core.Value{},
		properties:  map[string]core.Value{},
		typ:         typ,
	}
	if db.prevActor != "" {
		if prev := db.actors[db.prevActor]; prev != nil {
			prev.nextActor = id
		}
	}
	if db.currentBox != nil {
		db.currentBox.actorKeys = append(db.currentBox.actorKeys, id)
	}
	db.prevActor = id
}

func (db *seqDb) addSignal(from, to string, message *seqText, typ int, activate bool, central int) {
	// Mermaid recounts every earlier message here. A running count per participant says the same.
	if typ == lineActiveEnd {
		count := 0
		if from != "" {
			count = db.active[from]
		}
		if count < 1 {
			fail("Trying to inactivate an inactive participant (" + from + ")")
		}
		db.active[from] = count - 1
	} else if typ == lineActiveStart && from != "" {
		db.active[from]++
	}
	m := &seqMessage{id: strconv.Itoa(len(db.messages)), from: from, to: to, typ: typ, activate: activate, centralConnection: central}
	if message != nil {
		m.message = message.text
		m.wrap = message.wrap.or(db.autoWrap())
	} else {
		m.wrap = db.autoWrap()
	}
	db.messages = append(db.messages, m)
}

func (db *seqDb) addNote(actors []string, placement int, message *seqText) {
	wrap := message.wrap.or(db.autoWrap())
	db.notes = append(db.notes, &seqNote{actor: actors, placement: placement, message: message.text, wrap: wrap})
	from, to := actors[0], actors[0]
	if len(actors) > 1 {
		to = actors[1]
	}
	db.messages = append(db.messages, &seqMessage{
		id:        strconv.Itoa(len(db.messages)),
		from:      from,
		to:        to,
		message:   message.text,
		wrap:      wrap,
		typ:       lineNote,
		placement: placement,
	})
}

// addLinks: Mermaid logs and ignores menu text that is not valid JSON. It also undoes the `&amp;`
// and `&equals;` its sanitizer writes; Pele has no sanitizer, and a `;` cannot occur in the text.
func (db *seqDb) addLinks(actorID string, text *seqText) {
	actor := db.actors[actorID]
	if actor == nil {
		return
	}
	parsed, err := core.ParseJSON(text.text)
	if err != nil {
		return
	}
	keys, vals := entries(parsed)
	for i, k := range keys {
		actor.links[k] = vals[i]
	}
}

func (db *seqDb) addALink(actorID string, text *seqText) {
	actor := db.actors[actorID]
	if actor == nil {
		return
	}
	s := text.text
	sep := strings.IndexByte(s, '@')
	var key string
	if sep < 0 {
		// slice(0, -2)
		key = js.Slice(s, 0, -2)
		actor.links[js.Trim(key)] = js.Trim(s)
		return
	}
	key = js.Slice(s, 0, js.UnitsBefore(s, sep)-1)
	actor.links[js.Trim(key)] = js.Trim(s[sep+1:])
}

func (db *seqDb) addProperties(actorID string, text *seqText) {
	actor := db.actors[actorID]
	if actor == nil {
		return
	}
	parsed, err := core.ParseJSON(text.text)
	if err != nil {
		return
	}
	keys, vals := entries(parsed)
	for i, k := range keys {
		actor.properties[k] = vals[i]
	}
}

// addDetails: Mermaid reads the properties and links from a page element with this id. Pele
// keeps the id.
func (db *seqDb) addDetails(actorID string, text *seqText) {
	if actor := db.actors[actorID]; actor != nil {
		actor.details = text.text
	}
}

func (db *seqDb) apply(list []*statement) {
	for _, s := range list {
		db.applyOne(s)
	}
}

func (db *seqDb) applyOne(param *statement) {
	actor := param.actor
	switch param.typ {
	case "sequenceIndex":
		db.messages = append(db.messages, &seqMessage{
			id:        strconv.Itoa(len(db.messages)),
			numbering: &seqNumbering{start: param.sequenceIndex, step: param.sequenceIndexStep, visible: param.sequenceVisible},
			typ:       param.signalType,
		})
	case "addParticipant":
		db.addActor(actor, actor, param.descr, param.draw, param.config, param.hasConfig)
	case "createParticipant":
		if _, ok := db.actors[actor]; ok {
			fail("It is not possible to have actors with the same id, even if one is destroyed before the next is created. Use 'AS' aliases to simulate the behavior")
		}
		db.lastCreated = actor
		db.addActor(actor, actor, param.descr, param.draw, param.config, param.hasConfig)
		db.createdActors[actor] = len(db.messages)
	case "destroyParticipant":
		db.lastDestroyed = actor
		db.destroyedActors[actor] = len(db.messages)
	case "activeStart", "activeEnd", "centralConnection", "centralConnectionReverse":
		db.addSignal(actor, "", nil, param.signalType, false, 0)
	case "addNote":
		db.addNote(param.actors, param.placement, param.text)
	case "addLinks":
		db.addLinks(actor, param.text)
	case "addALink":
		db.addALink(actor, param.text)
	case "addProperties":
		db.addProperties(actor, param.text)
	case "addDetails":
		db.addDetails(actor, param.text)
	case "addMessage":
		if db.lastCreated != "" {
			if param.to != db.lastCreated {
				fail("The created participant " + db.lastCreated + " does not have an associated creating message after its declaration. Please check the sequence diagram.")
			}
			db.lastCreated = ""
		} else if db.lastDestroyed != "" {
			if param.to != db.lastDestroyed && param.from != db.lastDestroyed {
				fail("The destroyed participant " + db.lastDestroyed + " does not have an associated destroying message after its declaration. Please check the sequence diagram.")
			}
			db.lastDestroyed = ""
		}
		db.addSignal(param.from, param.to, param.text, param.signalType, param.activate, param.central)
	case "boxStart":
		db.addBox(param.boxData)
	case "boxEnd":
		db.currentBox = nil
	case "loopStart", "rectStart", "optStart", "altStart", "else", "parStart", "and", "criticalStart", "option", "breakStart":
		db.addSignal("", "", param.text, param.signalType, false, 0)
	case "loopEnd", "rectEnd", "optEnd", "altEnd", "parEnd", "criticalEnd", "breakEnd":
		db.addSignal("", "", nil, param.signalType, false, 0)
	}
}
