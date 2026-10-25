package parser

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/birdie-ai/katschema/parser/token"
)

func FuzzLexer(f *testing.F) {
	for _, src := range []string{
		"",
		"\xff",
		"a\xff",
		`"\uD800"`,
		`"\u12`,
		"1e+",
		"/*",
		"//\xff",
		"x--1",
		"#{(int),}",
		"\u00a0λ2",
		"/**//**/",
		"//\x00\xff\r\n/*λ*/",
		"a/ /*x*/ /b",
		"/* /* */ */",
		`"//"/**/"/* */"`,
	} {
		f.Add(src)
	}
	f.Fuzz(func(t *testing.T, src string) {
		l := lexer{src: src}
		for {
			before := l.off
			x, err := l.next()
			if err != nil {
				checkLexError(t, src, err)
				return
			}
			checkLexeme(t, src, before, l.off, x)
			if x.kind == token.EOF {
				checkEOF(t, &l, x)
				return
			}
		}
	})
}

func FuzzLexerJSON(f *testing.F) {
	// NOTE(i4k): this fuzzer works only on valid JSON corpus.

	for _, src := range []string{
		"null",
		"true",
		"0",
		"-0",
		"1e99999",
		`"\uD800"`,
		`[{},[],1,"a"]`,
		`{"a":1,"a":2}`,
	} {
		f.Add(src)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if json.Valid([]byte(src)) {
			checkLex(t, src)
		}
		quoted, err := json.Marshal(src)
		if err != nil {
			t.Fatal(err)
		}
		checkLex(t, string(quoted))
	})
}

func FuzzParse(f *testing.F) {
	for _, src := range []string{
		"", "null", "-0", "1e99999", `"\ud800"`, "\"\xff\"", "[1,]", `{"a":1,"a":2}`,
		`{"name":(string),"age":(int)}`, "(.a.true)", "(a /* comment */ . b)",
		"/* lead */ [(int)] // tail", "1true", "[0,1true]", "(int, optional)", "(int | string)",
	} {
		f.Add(src)
	}

	f.Fuzz(func(t *testing.T, src string) {
		tree, root, err := Parse(src)
		if err != nil {
			if tree != nil || root != 0 {
				t.Fatal("failed parse returned a tree")
			}
			var syntax *Error
			if !errors.As(err, &syntax) || syntax.Message == "" || !syntax.Span.Valid() || syntax.Span.End.Offset() > len(src) {
				t.Fatalf("invalid parse error: %#v", err)
			}
			return
		}
		if tree == nil || root == 0 {
			t.Fatal("successful parse returned no tree")
		}
		checkParsePrint(t, tree, root)
	})
}
