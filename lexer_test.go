package katschema

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// lexical test cases
type lextc struct {
	name string
	in   string
	toks tokvals
	vals []string
	err  error
}

func TestLexer(t *testing.T) {
	t.Parallel()
	for _, tc := range []lextc{
		{name: "empty input"},
		{
			name: "null alone",
			in:   `null`,
			toks: tokvals{tokval{kind: nul, end: 4}},
			vals: []string{"null"},
		},
		{
			name: "zero",
			in:   `0`,
			toks: tokvals{tokval{kind: num, end: 1}},
			vals: []string{"0"},
		},
		{
			name: "incomplete negative number",
			in:   `-`,
			err:  errLexNumber,
		},
		{
			name: "- followed by nondigit",
			in:   `-a`,
			err:  errLexNumber,
		},
		{
			name: "negative zero",
			in:   `-0`,
			toks: tokvals{tokval{kind: num, end: 2}},
			vals: []string{"-0"},
		},
		{
			name: "negative integer",
			in:   `-1337`,
			toks: tokvals{tokval{kind: num, end: 5}},
			vals: []string{"-1337"},
		},
		{
			name: "integer",
			in:   `1337`,
			toks: tokvals{tokval{kind: num, end: 4}},
			vals: []string{"1337"},
		},
		{
			name: "begin with empty comment",
			in:   `//`,
			toks: tokvals{
				tokval{
					kind:  com,
					start: 2,
					end:   2,
				},
			},
			vals: []string{
				"",
			},
		},
		{
			name: "empty comment after ws",
			in:   `	//`,
			toks: tokvals{
				tokval{
					kind:  com,
					start: 3,
					end:   3,
				},
			},
			vals: []string{""},
		},
		{
			name: "comment",
			in:   `	//test`,
			toks: tokvals{
				tokval{
					kind:  com,
					start: 3,
					end:   7,
				},
			},
			vals: []string{"test"},
		},
		{
			name: "comment containing //",
			in:   `	//test //`,
			toks: tokvals{
				tokval{
					kind:  com,
					start: 3,
					end:   10,
				},
			},
			vals: []string{"test //"},
		},
		{
			name: "comment followed by \r",
			in:   "	//test\r",
			toks: tokvals{
				tokval{
					kind:  com,
					start: 3,
					end:   8,
				},
			},
			vals: []string{"test\r"},
		},
		{
			name: "comment followed by \n",
			in:   "	//test\n",
			toks: tokvals{
				tokval{
					kind:  com,
					start: 3,
					end:   7,
				},
			},
			vals: []string{"test"},
		},
	} {
		lextest(t, tc)
	}
}

func lextest(t *testing.T, tc lextc) {
	t.Run(tc.name, func(t *testing.T) {
		lex := newlexer([]byte(tc.in))
		var gottoks tokvals
		var gotvals []string
		for {
			err := lex.Next()
			assertError(t, tc.err, err)
			if err != nil {
				break
			}
			tok := lex.Token()
			if tok.kind == eof {
				break
			}
			gottoks = append(gottoks, tok)
			gotvals = append(gotvals, lex.Text())
		}
		assert(t, tc.toks, gottoks)
		assert(t, tc.vals, gotvals)
	})
}

func TestLexString(t *testing.T) {
	t.Parallel()
}

func assertError(t *testing.T, want, got error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("errors differ: [%v] != [%v]", want, got)
	}
}

func assert(t *testing.T, want, got any) {
	t.Helper()
	if diff := cmp.Diff(want, got, cmpopts.EquateComparable(tokval{})); diff != "" {
		t.Fatal(diff)
	}
}
