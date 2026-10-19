package core

import (
	"strconv"
	"strings"

	"github.com/alessioalex/pele/go/internal/js"
)

// ErrorCode classifies an Error.
type ErrorCode string

const (
	CodeUnsupported ErrorCode = "unsupported-diagram"
	CodeSyntax      ErrorCode = "syntax"
	CodeSemantic    ErrorCode = "semantic"
	CodeLimit       ErrorCode = "limit"
)

// Error is PeleError. Type and MermaidType are empty when unknown. Line and Column count from 1,
// with columns in UTF-16 code units as in the TypeScript library; both are 0 when unknown.
type Error struct {
	Message     string
	Code        ErrorCode
	Type        string
	MermaidType string
	Line        int
	Column      int
	Snippet     string
}

func (e *Error) Error() string { return e.Message }

// NewError makes an error with the given code and no position.
func NewError(message string, code ErrorCode) *Error {
	return &Error{Message: message, Code: code}
}

// Fail panics with a PeleError. The engine recovers it at the diagram boundary, so that ported
// code can throw the way the TypeScript does.
func Fail(message string, code ErrorCode) {
	panic(NewError(message, code))
}

// Throw panics with err.
func Throw(err *Error) { panic(err) }

// Location is a position in source text.
type Location struct {
	Line    int
	Column  int
	Snippet string
}

// Locate finds the line and column of byte offset in src, with a snippet that points at it.
func Locate(src string, offset int) Location {
	// Past the end, the column keeps counting, as it does in the TypeScript.
	past := 0
	if offset > len(src) {
		past = offset - len(src)
		offset = len(src)
	}
	if offset < 0 {
		offset = 0
	}
	line := 1
	lineStart := 0
	for i := strings.IndexByte(src, '\n'); i != -1 && i < offset; {
		line++
		lineStart = i + 1
		next := strings.IndexByte(src[i+1:], '\n')
		if next == -1 {
			break
		}
		i += 1 + next
	}
	lineEnd := strings.IndexByte(src[offset:], '\n')
	if lineEnd == -1 {
		lineEnd = len(src)
	} else {
		lineEnd += offset
	}
	column := js.Len(src[lineStart:offset]) + past + 1
	text := src[lineStart:lineEnd]
	return Location{Line: line, Column: column, Snippet: text + "\n" + strings.Repeat("-", column-1) + "^"}
}

// LocateUnits is Locate for an offset in UTF-16 code units.
func LocateUnits(src string, offset int) Location {
	return Locate(src, js.ByteOffset(src, offset))
}

// SyntaxError is the error a Jison parser reports, for a byte offset in src.
func SyntaxError(typ, src string, offset int, detail string, lexical bool) *Error {
	loc := Locate(src, offset)
	var message string
	if lexical {
		message = "Lexical error on line " + strconv.Itoa(loc.Line) + ". Unrecognized text.\n" + loc.Snippet
	} else {
		message = "Parse error on line " + strconv.Itoa(loc.Line) + ":\n" + loc.Snippet + "\n" + detail
	}
	return &Error{Message: message, Code: CodeSyntax, Type: typ, Line: loc.Line, Column: loc.Column, Snippet: loc.Snippet}
}
