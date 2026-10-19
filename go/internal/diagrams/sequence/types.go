//go:build wip

package sequence

import "github.com/alessioalex/pele/go/internal/core"

// The message kinds, numbered as in Mermaid.
const (
	lineSolid                         = 0
	lineDotted                        = 1
	lineNote                          = 2
	lineSolidCross                    = 3
	lineDottedCross                   = 4
	lineSolidOpen                     = 5
	lineDottedOpen                    = 6
	lineLoopStart                     = 10
	lineLoopEnd                       = 11
	lineAltStart                      = 12
	lineAltElse                       = 13
	lineAltEnd                        = 14
	lineOptStart                      = 15
	lineOptEnd                        = 16
	lineActiveStart                   = 17
	lineActiveEnd                     = 18
	lineParStart                      = 19
	lineParAnd                        = 20
	lineParEnd                        = 21
	lineRectStart                     = 22
	lineRectEnd                       = 23
	lineSolidPoint                    = 24
	lineDottedPoint                   = 25
	lineAutonumber                    = 26
	lineCriticalStart                 = 27
	lineCriticalOption                = 28
	lineCriticalEnd                   = 29
	lineBreakStart                    = 30
	lineBreakEnd                      = 31
	lineParOverStart                  = 32
	lineBidirectionalSolid            = 33
	lineBidirectionalDotted           = 34
	lineSolidTop                      = 41
	lineSolidBottom                   = 42
	lineStickTop                      = 43
	lineStickBottom                   = 44
	lineSolidArrowTopReverse          = 45
	lineSolidArrowBottomReverse       = 46
	lineStickArrowTopReverse          = 47
	lineStickArrowBottomReverse       = 48
	lineSolidTopDotted                = 51
	lineSolidBottomDotted             = 52
	lineStickTopDotted                = 53
	lineStickBottomDotted             = 54
	lineSolidArrowTopReverseDotted    = 55
	lineSolidArrowBottomReverseDotted = 56
	lineStickArrowTopReverseDotted    = 57
	lineStickArrowBottomReverseDotted = 58
	lineCentralConnection             = 59
	lineCentralConnectionReverse      = 60
	lineCentralConnectionDual         = 61
	lineTypes                         = 62
)

const (
	placeLeftOf  = 0
	placeRightOf = 1
	placeOver    = 2
)

// tri is a boolean that may be undefined.
type tri int8

const (
	triUndefined tri = iota
	triFalse
	triTrue
)

// or is `wrap ?? def`.
func (w tri) or(def bool) bool {
	if w == triUndefined {
		return def
	}
	return w == triTrue
}

type seqText struct {
	text string
	wrap tri
}

type seqBoxData struct {
	// text is "" for undefined: a box title is never the empty string.
	text  string
	color string
	wrap  tri
}

type seqBox struct {
	name      string
	wrap      bool
	fill      string
	actorKeys []string
}

type seqActor struct {
	box         *seqBox
	name        string
	description string
	wrap        bool
	prevActor   string
	nextActor   string
	links       map[string]core.Value
	properties  map[string]core.Value
	// The id of the page element named by a `details` statement. Pele has no page to read it from.
	details string
	typ     string
}

type seqNumbering struct {
	// 0 is undefined, which the renderer treats alike.
	start   float64
	step    float64
	visible bool
}

// seqMessage is a message. An empty from or to is undefined: participant ids are never empty.
type seqMessage struct {
	id   string
	from string
	to   string
	// numbering is set instead of message for an autonumber statement.
	message           string
	numbering         *seqNumbering
	wrap              bool
	typ               int
	activate          bool
	placement         int
	centralConnection int
}

type seqNote struct {
	actor     []string
	placement int
	message   string
	wrap      bool
}

// statement is one entry of the statement list the parser hands to apply, as Mermaid's grammar
// builds it. The texts that Mermaid keeps under a key per statement type (msg, loopText, altText,
// ...) are all in text.
type statement struct {
	typ        string
	actor      string
	actors     []string
	draw       string
	descr      *seqText
	config     string
	hasConfig  bool
	signalType int
	from       string
	to         string
	text       *seqText
	activate   bool
	central    int
	placement  int
	boxData    *seqBoxData
	// autonumber
	sequenceIndex     float64
	sequenceIndexStep float64
	sequenceVisible   bool
}
