//go:build wip

package sequence

import (
	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/js"
)

var signalOf = func() (s [tCount]int8) {
	for i := range s {
		s[i] = -1
	}
	s[tSolidOpenArrow] = lineSolidOpen
	s[tDottedOpenArrow] = lineDottedOpen
	s[tSolidArrow] = lineSolid
	s[tDottedArrow] = lineDotted
	s[tBidirectionalSolidArrow] = lineBidirectionalSolid
	s[tBidirectionalDottedArrow] = lineBidirectionalDotted
	s[tSolidCross] = lineSolidCross
	s[tDottedCross] = lineDottedCross
	s[tSolidPoint] = lineSolidPoint
	s[tDottedPoint] = lineDottedPoint
	s[tSolidArrowTop] = lineSolidTop
	s[tSolidArrowBottom] = lineSolidBottom
	s[tStickArrowTop] = lineStickTop
	s[tStickArrowBottom] = lineStickBottom
	s[tSolidArrowTopDotted] = lineSolidTopDotted
	s[tSolidArrowBottomDotted] = lineSolidBottomDotted
	s[tStickArrowTopDotted] = lineStickTopDotted
	s[tStickArrowBottomDotted] = lineStickBottomDotted
	s[tSolidArrowTopReverse] = lineSolidArrowTopReverse
	s[tSolidArrowBottomReverse] = lineSolidArrowBottomReverse
	s[tStickArrowTopReverse] = lineStickArrowTopReverse
	s[tStickArrowBottomReverse] = lineStickArrowBottomReverse
	s[tSolidArrowTopReverseDotted] = lineSolidArrowTopReverseDotted
	s[tSolidArrowBottomReverseDotted] = lineSolidArrowBottomReverseDotted
	s[tStickArrowTopReverseDotted] = lineStickArrowTopReverseDotted
	s[tStickArrowBottomReverseDotted] = lineStickArrowBottomReverseDotted
	return
}()

const (
	kTop = iota
	kBox
	kLoop
	kRect
	kOpt
	kBreak
	kAlt
	kPar
	kCritical
)

var blockOf = map[int]int{
	tLoop: kLoop, tRect: kRect, tOpt: kOpt, tBreak: kBreak, tAlt: kAlt, tPar: kPar, tParOver: kPar,
	tCritical: kCritical, tBox: kBox,
}

const signalTokens = "'SOLID_OPEN_ARROW', 'DOTTED_OPEN_ARROW', 'SOLID_ARROW', 'DOTTED_ARROW', 'SOLID_CROSS', 'DOTTED_CROSS', 'SOLID_POINT', 'DOTTED_POINT'"

type parser struct {
	src  string
	tk   tokenList
	i    int
	db   *seqDb
	list []*statement
}

func (p *parser) kind(j int) int {
	if j < len(p.tk.types) {
		return p.tk.types[j]
	}
	return -1
}

func (p *parser) fail(expected string) {
	t := p.tk.types[p.i]
	at := js.ByteOffset(p.src, p.tk.starts[p.i])
	if t == tERROR {
		panic(core.SyntaxError("sequence", p.src, at, "", true))
	}
	panic(core.SyntaxError("sequence", p.src, at, "Expecting "+expected+", got '"+tokenNames[t]+"'", false))
}

func (p *parser) expect(typ int) string {
	if p.tk.types[p.i] != typ {
		p.fail("'" + tokenNames[typ] + "'")
	}
	p.i++
	return p.tk.texts[p.i-1]
}

func (p *parser) push(s *statement) { p.list = append(p.list, s) }

func (p *parser) actor() *statement {
	t := p.tk.types[p.i]
	if t != tACTOR && t != tLink {
		p.fail("'ACTOR'")
	}
	p.i++
	return &statement{typ: "addParticipant", actor: p.tk.texts[p.i-1]}
}

func (p *parser) text2() *seqText {
	t := js.Trim(p.expect(tTXT))
	return p.db.parseMessage(t[1:])
}

func (p *parser) participant() *statement {
	t := p.tk.types[p.i]
	p.i++
	if t == tDestroy {
		node := p.actor()
		p.expect(tNEWLINE)
		node.typ = "destroyParticipant"
		return node
	}
	node := p.actor()
	if p.kind(p.i) == tConfigStart && p.tk.types[p.i-1] == tACTOR {
		p.i++
		content := p.expect(tConfigContent)
		p.expect(tConfigEnd)
		node.config = js.Trim(content)
		node.hasConfig = true
	}
	if t == tParticipant {
		node.draw = "participant"
	} else {
		node.draw = "actor"
	}
	switch p.kind(p.i) {
	case tAS:
		p.i++
		alias := p.expect(tRestOfLine)
		p.expect(tNEWLINE)
		node.descr = p.db.parseMessage(alias)
	case tNEWLINE:
		p.i++
	default:
		p.fail("'AS', 'NEWLINE'")
	}
	return node
}

func (p *parser) signal(from *statement) {
	centralFrom := false
	if p.kind(p.i) == tCENTRAL {
		centralFrom = true
		p.i++
	}
	signalType := int(signalOf[p.tk.types[p.i]])
	if signalType < 0 {
		p.fail(signalTokens)
	}
	p.i++
	mark := p.kind(p.i)
	if mark == tCENTRAL || (!centralFrom && (mark == tPLUS || mark == tMINUS)) {
		p.i++
	}
	to := p.actor()
	msg := p.text2()
	message := &statement{typ: "addMessage", from: from.actor, to: to.actor, signalType: signalType, text: msg}
	p.push(from)
	p.push(to)
	p.push(message)
	switch {
	case centralFrom:
		dual := mark == tCENTRAL
		message.activate = dual
		if dual {
			message.central = lineCentralConnectionDual
			p.push(&statement{typ: "centralConnection", signalType: lineCentralConnection, actor: to.actor})
		} else {
			message.central = lineCentralConnectionReverse
		}
		p.push(&statement{typ: "centralConnectionReverse", signalType: lineCentralConnectionReverse, actor: from.actor})
	case mark == tPLUS:
		message.activate = true
		p.push(&statement{typ: "activeStart", signalType: lineActiveStart, actor: to.actor})
	case mark == tMINUS:
		p.push(&statement{typ: "activeEnd", signalType: lineActiveEnd, actor: from.actor})
	case mark == tCENTRAL:
		message.activate = true
		message.central = lineCentralConnection
		p.push(&statement{typ: "centralConnection", signalType: lineCentralConnection, actor: to.actor})
	}
}

func (p *parser) menu(typ string) {
	p.i++
	who := p.actor()
	p.push(who)
	p.push(&statement{typ: typ, actor: who.actor, text: p.text2()})
}

func (p *parser) note() {
	p.i++
	t := p.kind(p.i)
	if t == tOver {
		p.i++
		first := p.actor()
		p.push(first)
		ids := []string{first.actor, first.actor}
		if p.kind(p.i) == tCOMMA {
			p.i++
			second := p.actor()
			p.push(second)
			ids = []string{first.actor, second.actor}
		}
		p.push(&statement{typ: "addNote", placement: placeOver, actors: ids, text: p.text2()})
		return
	}
	if t != tLeftOf && t != tRightOf {
		p.fail("'over', 'left_of', 'right_of'")
	}
	p.i++
	who := p.actor()
	p.push(who)
	placement := placeRightOf
	if t == tLeftOf {
		placement = placeLeftOf
	}
	p.push(&statement{typ: "addNote", placement: placement, actor: who.actor, actors: []string{who.actor}, text: p.text2()})
}

func (p *parser) autonumber() {
	p.i++
	node := &statement{typ: "sequenceIndex", sequenceVisible: true, signalType: lineAutonumber}
	switch p.kind(p.i) {
	case tNUM:
		node.sequenceIndex = js.ToNumber(p.tk.texts[p.i])
		p.i++
		node.sequenceIndexStep = 1
		if p.kind(p.i) == tNUM {
			node.sequenceIndexStep = js.ToNumber(p.tk.texts[p.i])
			p.i++
		}
	case tOff:
		p.i++
		node.sequenceVisible = false
	}
	if p.kind(p.i) != tNEWLINE {
		p.fail("'NEWLINE', 'NUM', 'off'")
	}
	p.i++
	p.push(node)
}

type block struct {
	kind      int
	startType int
}

// The start of a block.
func (p *parser) open(kind, startType int, text string) {
	switch kind {
	case kBox:
		p.push(&statement{typ: "boxStart", boxData: p.db.parseBoxData(text)})
	case kLoop:
		p.push(&statement{typ: "loopStart", text: p.db.parseMessage(text), signalType: lineLoopStart})
	case kRect:
		p.push(&statement{typ: "rectStart", text: p.db.parseMessage(text), signalType: lineRectStart})
	case kOpt:
		p.push(&statement{typ: "optStart", text: p.db.parseMessage(text), signalType: lineOptStart})
	case kBreak:
		p.push(&statement{typ: "breakStart", text: p.db.parseMessage(text), signalType: lineBreakStart})
	case kAlt:
		p.push(&statement{typ: "altStart", text: p.db.parseMessage(text), signalType: lineAltStart})
	case kPar:
		p.push(&statement{typ: "parStart", text: p.db.parseMessage(text), signalType: startType})
	default:
		p.push(&statement{typ: "criticalStart", text: p.db.parseMessage(text), signalType: lineCriticalStart})
	}
}

func (p *parser) close(kind int) {
	switch kind {
	case kBox:
		p.push(&statement{typ: "boxEnd"})
	case kLoop:
		p.push(&statement{typ: "loopEnd", signalType: lineLoopEnd})
	case kRect:
		p.push(&statement{typ: "rectEnd", signalType: lineRectEnd})
	case kOpt:
		p.push(&statement{typ: "optEnd", signalType: lineOptEnd})
	case kBreak:
		p.push(&statement{typ: "breakEnd", signalType: lineBreakEnd})
	case kAlt:
		p.push(&statement{typ: "altEnd", signalType: lineAltEnd})
	case kPar:
		p.push(&statement{typ: "parEnd", signalType: lineParEnd})
	default:
		p.push(&statement{typ: "criticalEnd", signalType: lineCriticalEnd})
	}
}

// parseSequence reads a sequence diagram into db. Mermaid's grammar nests the statements of each
// block in lists that its model builder walks depth first; Pele keeps them in one list in the
// order that walk visits them.
func parseSequence(src string, db *seqDb) {
	p := &parser{src: src, tk: tokenize(src), db: db}

	for p.kind(p.i) == tNEWLINE {
		p.i++
	}
	p.expect(tSD)

	var stack []block
	frame := block{kind: kTop}

	for {
		t := p.tk.types[p.i]
		if frame.kind == kBox {
			switch t {
			case tParticipant, tParticipantActor, tDestroy:
				p.push(p.participant())
			case tNEWLINE:
				p.i++
			case tEnd:
				p.i++
				p.close(frame.kind)
				frame = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			default:
				p.fail("'end', 'NEWLINE', 'participant', 'participant_actor', 'destroy'")
			}
			continue
		}
		switch t {
		case tNEWLINE, tINVALID:
			p.i++
		case tParticipant, tParticipantActor, tDestroy:
			p.push(p.participant())
		case tCreate:
			p.i++
			next := p.kind(p.i)
			if next != tParticipant && next != tParticipantActor && next != tDestroy {
				p.fail("'participant', 'participant_actor', 'destroy'")
			}
			node := p.participant()
			node.typ = "createParticipant"
			p.push(node)
		case tACTOR:
			p.signal(p.actor())
			p.expect(tNEWLINE)
		case tLink:
			next := p.kind(p.i + 1)
			if next == tACTOR || next == tLink {
				p.menu("addALink")
			} else {
				p.signal(p.actor())
			}
			p.expect(tNEWLINE)
		case tLinks:
			p.menu("addLinks")
			p.expect(tNEWLINE)
		case tProperties:
			p.menu("addProperties")
			p.expect(tNEWLINE)
		case tDetails:
			p.menu("addDetails")
			p.expect(tNEWLINE)
		case tNote:
			p.note()
			p.expect(tNEWLINE)
		case tAutonumber:
			p.autonumber()
		case tActivate, tDeactivate:
			p.i++
			who := p.actor()
			p.expect(tNEWLINE)
			if t == tActivate {
				p.push(&statement{typ: "activeStart", signalType: lineActiveStart, actor: who.actor})
			} else {
				p.push(&statement{typ: "activeEnd", signalType: lineActiveEnd, actor: who.actor})
			}
		case tTitle, tLegacyTitle:
			skip := 6
			if t == tLegacyTitle {
				skip = 7
			}
			db.setDiagramTitle(js.SliceFrom(p.tk.texts[p.i], skip))
			p.i++
		case tAccTitle:
			p.i++
			db.setAccTitle(js.Trim(p.expect(tAccTitleValue)))
		case tAccDescr:
			p.i++
			db.setAccDescription(js.Trim(p.expect(tAccDescrValue)))
		case tAccDescrMultilineValue:
			db.setAccDescription(js.Trim(p.tk.texts[p.i]))
			p.i++
		case tLoop, tRect, tOpt, tBreak, tAlt, tPar, tParOver, tCritical, tBox:
			p.i++
			text := p.expect(tRestOfLine)
			stack = append(stack, frame)
			frame = block{kind: blockOf[t], startType: lineParStart}
			if t == tParOver {
				frame.startType = lineParOverStart
			}
			p.open(frame.kind, frame.startType, text)
		case tElse, tAnd, tOption:
			want := kCritical
			typ, signalType := "option", lineCriticalOption
			if t == tElse {
				want, typ, signalType = kAlt, "else", lineAltElse
			} else if t == tAnd {
				want, typ, signalType = kPar, "and", lineParAnd
			}
			if frame.kind != want {
				p.fail("'end'")
			}
			p.i++
			text := p.expect(tRestOfLine)
			p.push(&statement{typ: typ, signalType: signalType, text: p.db.parseMessage(text)})
		case tEnd:
			if frame.kind == kTop {
				p.fail("'NEWLINE', 'EOF'")
			}
			p.i++
			p.close(frame.kind)
			frame = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		case tEND:
			if frame.kind != kTop {
				p.fail("'end'")
			}
			db.apply(p.list)
			return
		default:
			p.fail("'NEWLINE', 'ACTOR', 'participant', 'note', 'end'")
		}
	}
}
