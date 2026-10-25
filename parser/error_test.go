package parser

import (
	"testing"

	"github.com/birdie-ai/katschema/parser/token"
)

func TestErrorPosition(t *testing.T) {
	type testcase struct {
		span token.Span
		want string
	}
	for _, tc := range []testcase{
		{span: token.NewSpan(0, 0), want: "parser: byte 0: expected value"},
		{span: token.NewSpan(7, 8), want: "parser: byte 7: expected value"},
		{span: token.UnknownSpan(), want: "parser: expected value"},
	} {
		err := &Error{Span: tc.span, Message: "expected value"}
		if got := err.Error(); got != tc.want {
			t.Errorf("span %v: error = %q, want %q", tc.span, got, tc.want)
		}
	}
}
