package parser

import (
	"fmt"

	"github.com/birdie-ai/katschema/parser/token"
)

// Error reports a syntax error at a byte range in the source.
type Error struct {
	Span    token.Span
	Message string
}

func (e *Error) Error() string {
	if !e.Span.Valid() {
		return "parser: " + e.Message
	}
	return fmt.Sprintf("parser: byte %d: %s", e.Span.Start, e.Message)
}

func syntaxError(start, end int, message string) error {
	return &Error{Span: token.NewSpan(start, end), Message: message}
}
