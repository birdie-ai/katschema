package parser

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/birdie-ai/katschema/parser/token"
)

func TestLexerTokens(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src  string
		kind token.Kind
	}

	for _, tc := range []testcase{
		{src: "(", kind: token.LParen},
		{src: ")", kind: token.RParen},
		{src: "[", kind: token.LBrack},
		{src: "]", kind: token.RBrack},
		{src: "{", kind: token.LBrace},
		{src: "}", kind: token.RBrace},
		{src: "#{", kind: token.TupleOpen},
		{src: ",", kind: token.Comma},
		{src: ":", kind: token.Colon},
		{src: ".", kind: token.Dot},
		{src: "=", kind: token.Assign},
		{src: "==", kind: token.Eq},
		{src: "!=", kind: token.Ne},
		{src: "<", kind: token.Lt},
		{src: "<=", kind: token.Le},
		{src: ">", kind: token.Gt},
		{src: ">=", kind: token.Ge},
		{src: "~", kind: token.Match},
		{src: "&&", kind: token.And},
		{src: "||", kind: token.Or},
		{src: "!", kind: token.Not},
		{src: "+", kind: token.Add},
		{src: "-", kind: token.Sub},
		{src: "*", kind: token.Mul},
		{src: "/", kind: token.Div},
		{src: "%", kind: token.Mod},
		{src: "|", kind: token.Pipe},
		{src: "//", kind: token.Comment},
		{src: "// text", kind: token.Comment},
		{src: "//\x00\xff", kind: token.Comment},
		{src: "/**/", kind: token.Comment},
		{src: "/* text */", kind: token.Comment},
		{src: "/*\x00\xff*/", kind: token.Comment},
		{src: "/*//*/", kind: token.Comment},
		{src: "// /* */", kind: token.Comment},
		{src: "true", kind: token.True},
		{src: "false", kind: token.False},
		{src: "null", kind: token.Null},
		{src: "in", kind: token.In},
		{src: "_", kind: token.Ident},
		{src: "_1", kind: token.Ident},
		{src: "foo_9", kind: token.Ident},
		{src: "True", kind: token.Ident},
		{src: "False", kind: token.Ident},
		{src: "Null", kind: token.Ident},
		{src: "IN", kind: token.Ident},
		{src: "integer", kind: token.Ident},
		{src: "truex", kind: token.Ident},
		{src: "in1", kind: token.Ident},
		{src: "trueé", kind: token.Ident},
		{src: "λ2", kind: token.Ident},
		{src: "牦牛_9", kind: token.Ident},
		{src: "𐐀x", kind: token.Ident},
		{src: "0", kind: token.Int},
		{src: "1", kind: token.Int},
		{src: "10", kind: token.Int},
		{src: strings.Repeat("9", 1024), kind: token.Int},
		{src: "0.0", kind: token.Decimal},
		{src: "1e0", kind: token.Decimal},
		{src: "1E+09", kind: token.Decimal},
		{src: "1e-9", kind: token.Decimal},
		{src: "12.34E-56", kind: token.Decimal},
		{src: "1e" + strings.Repeat("9", 1024), kind: token.Decimal},
		{src: `""`, kind: token.String},
		{src: `"text"`, kind: token.String},
		{src: `"\"\\\/\b\f\n\r\t"`, kind: token.String},
		{src: `"\u0000"`, kind: token.String},
		{src: `"\u00e9"`, kind: token.String},
		{src: `"\uD834\uDD1E"`, kind: token.String},
		{src: `"\uD800"`, kind: token.String},
		{src: `"\uDC00"`, kind: token.String},
		{src: `"\uD800\u0041"`, kind: token.String},
		{src: `"去他妈的人工智能🖕"`, kind: token.String},
		{src: `"// comment"`, kind: token.String},
		{src: `"/* comment */"`, kind: token.String},
		{src: "\"\xff\"", kind: token.String},
		{src: "\"\xc0\xaf\"", kind: token.String},
	} {
		t.Run(tc.kind.String()+"/"+tc.src, func(t *testing.T) {
			got := checkLex(t, tc.src)
			want := lexeme{kind: tc.kind, span: token.NewSpan(0, len(tc.src))}
			if len(got) != 1 || got[0] != want {
				t.Fatalf("tokens = %#v, want [%#v]", got, want)
			}
		})
	}
}

func TestLexerSequences(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src  string
		want []lexeme
	}
	for _, tc := range []testcase{
		{
			src:  "x-10",
			want: []lexeme{{kind: token.Ident, span: token.NewSpan(0, 1)}, {kind: token.Sub, span: token.NewSpan(1, 2)}, {kind: token.Int, span: token.NewSpan(2, 4)}},
		},
		{
			src:  "x - 10",
			want: []lexeme{{kind: token.Ident, span: token.NewSpan(0, 1)}, {kind: token.Sub, span: token.NewSpan(2, 3)}, {kind: token.Int, span: token.NewSpan(4, 6)}},
		},
		{
			src:  "x--10",
			want: []lexeme{{kind: token.Ident, span: token.NewSpan(0, 1)}, {kind: token.Sub, span: token.NewSpan(1, 2)}, {kind: token.Sub, span: token.NewSpan(2, 3)}, {kind: token.Int, span: token.NewSpan(3, 5)}},
		},
		{
			src:  "-0",
			want: []lexeme{{kind: token.Sub, span: token.NewSpan(0, 1)}, {kind: token.Int, span: token.NewSpan(1, 2)}},
		},
		{
			src:  "-1.0e-2",
			want: []lexeme{{kind: token.Sub, span: token.NewSpan(0, 1)}, {kind: token.Decimal, span: token.NewSpan(1, 7)}},
		},
		{
			src:  "true false null in",
			want: []lexeme{{kind: token.True, span: token.NewSpan(0, 4)}, {kind: token.False, span: token.NewSpan(5, 10)}, {kind: token.Null, span: token.NewSpan(11, 15)}, {kind: token.In, span: token.NewSpan(16, 18)}},
		},
		{
			src:  "1true",
			want: []lexeme{{kind: token.Int, span: token.NewSpan(0, 1)}, {kind: token.True, span: token.NewSpan(1, 5)}},
		},
		{
			src:  `"a""b"`,
			want: []lexeme{{kind: token.String, span: token.NewSpan(0, 3)}, {kind: token.String, span: token.NewSpan(3, 6)}},
		},
		{
			src:  "a/*x*/b",
			want: []lexeme{{kind: token.Ident, span: token.NewSpan(0, 1)}, {kind: token.Comment, span: token.NewSpan(1, 6)}, {kind: token.Ident, span: token.NewSpan(6, 7)}},
		},
		{
			src:  "a//x\r\nb",
			want: []lexeme{{kind: token.Ident, span: token.NewSpan(0, 1)}, {kind: token.Comment, span: token.NewSpan(1, 5)}, {kind: token.Ident, span: token.NewSpan(6, 7)}},
		},
		{
			src: "/* outer /* inner */x", want: []lexeme{{kind: token.Comment, span: token.NewSpan(0, 20)}, {kind: token.Ident, span: token.NewSpan(20, 21)}},
		},
		{
			src:  "/**//**/",
			want: []lexeme{{kind: token.Comment, span: token.NewSpan(0, 4)}, {kind: token.Comment, span: token.NewSpan(4, 8)}},
		},
		{
			src:  "/**///tail",
			want: []lexeme{{kind: token.Comment, span: token.NewSpan(0, 4)}, {kind: token.Comment, span: token.NewSpan(4, 10)}},
		},
		{
			src:  "//a\n/*b*///c",
			want: []lexeme{{kind: token.Comment, span: token.NewSpan(0, 3)}, {kind: token.Comment, span: token.NewSpan(4, 9)}, {kind: token.Comment, span: token.NewSpan(9, 12)}},
		},
		{
			src:  "a/ /*x*/ /b",
			want: []lexeme{{kind: token.Ident, span: token.NewSpan(0, 1)}, {kind: token.Div, span: token.NewSpan(1, 2)}, {kind: token.Comment, span: token.NewSpan(3, 8)}, {kind: token.Div, span: token.NewSpan(9, 10)}, {kind: token.Ident, span: token.NewSpan(10, 11)}},
		},
		{
			src:  "/* /* */ */",
			want: []lexeme{{kind: token.Comment, span: token.NewSpan(0, 8)}, {kind: token.Mul, span: token.NewSpan(9, 10)}, {kind: token.Div, span: token.NewSpan(10, 11)}},
		},
		{
			src:  "λ//é\r\nx",
			want: []lexeme{{kind: token.Ident, span: token.NewSpan(0, 2)}, {kind: token.Comment, span: token.NewSpan(2, 7)}, {kind: token.Ident, span: token.NewSpan(8, 9)}},
		},
		{
			src:  "/*é\n世*/x",
			want: []lexeme{{kind: token.Comment, span: token.NewSpan(0, 10)}, {kind: token.Ident, span: token.NewSpan(10, 11)}},
		},
		{
			src:  "//é\u2028x\n",
			want: []lexeme{{kind: token.Comment, span: token.NewSpan(0, 8)}},
		},
		{
			src:  " // text\r\n",
			want: []lexeme{{kind: token.Comment, span: token.NewSpan(1, 9)}},
		},
		{
			src:  "//\n",
			want: []lexeme{{kind: token.Comment, span: token.NewSpan(0, 2)}},
		},
		{
			src:  `"//"/**/"/* */"`,
			want: []lexeme{{kind: token.String, span: token.NewSpan(0, 4)}, {kind: token.Comment, span: token.NewSpan(4, 8)}, {kind: token.String, span: token.NewSpan(8, 15)}},
		},
		{
			src: "|| | && == = != ! <= < >= >", want: []lexeme{
				{kind: token.Or, span: token.NewSpan(0, 2)}, {kind: token.Pipe, span: token.NewSpan(3, 4)}, {kind: token.And, span: token.NewSpan(5, 7)},
				{kind: token.Eq, span: token.NewSpan(8, 10)}, {kind: token.Assign, span: token.NewSpan(11, 12)}, {kind: token.Ne, span: token.NewSpan(13, 15)},
				{kind: token.Not, span: token.NewSpan(16, 17)}, {kind: token.Le, span: token.NewSpan(18, 20)}, {kind: token.Lt, span: token.NewSpan(21, 22)},
				{kind: token.Ge, span: token.NewSpan(23, 25)}, {kind: token.Gt, span: token.NewSpan(26, 27)},
			},
		},
		{
			src:  "\r\nλ\u00a0x",
			want: []lexeme{{kind: token.Ident, span: token.NewSpan(2, 4)}, {kind: token.Ident, span: token.NewSpan(6, 7)}},
		},
	} {
		t.Run(tc.src, func(t *testing.T) {
			got := checkLex(t, tc.src)
			if len(got) != len(tc.want) {
				t.Fatalf("tokens = %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("token %d = %#v, want %#v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestLexerWhitespace(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"",
		" \t\r\n\v\f",
		"\u0085\u00a0\u1680\u2000\u2028\u2029\u202f\u205f\u3000",
	} {
		t.Run("tc", func(t *testing.T) {
			if got := checkLex(t, src); len(got) != 0 {
				t.Fatalf("tokens = %#v, want no tokens", got)
			}
		})
	}
}

func TestLexerErrors(t *testing.T) {
	t.Parallel()

	type testcase struct {
		src        string
		start, end int
	}
	for _, tc := range []testcase{
		{src: "&", start: 0, end: 1},
		{src: "&&&", start: 2, end: 3},
		{src: "#", start: 0, end: 1},
		{src: "# {", start: 0, end: 1},
		{src: "@", start: 0, end: 1},
		{src: "\ufeff", start: 0, end: 3},
		{src: "a\u0301", start: 1, end: 3},
		{src: "\u0661", start: 0, end: 2},
		{src: "a\u0661", start: 1, end: 3},
		{src: "1e\u0661", start: 2, end: 4},
		{src: "\x00", start: 0, end: 1},
		{src: "a\xff", start: 1, end: 2},
		{src: "1\xff", start: 1, end: 2},
		{src: "\xff", start: 0, end: 1},
		{src: "a١", start: 1, end: 3},
		{src: "١", start: 0, end: 2},
		{src: "😀", start: 0, end: 4},
		{src: "00", start: 1, end: 2},
		{src: "01", start: 1, end: 2},
		{src: "-01", start: 2, end: 3},
		{src: "1.", start: 2, end: 2},
		{src: "1.x", start: 2, end: 3},
		{src: "1e", start: 2, end: 2},
		{src: "1E", start: 2, end: 2},
		{src: "1e+", start: 3, end: 3},
		{src: "1e-", start: 3, end: 3},
		{src: "1e+x", start: 3, end: 4},
		{src: "1e++", start: 3, end: 4},
		{src: `"`, start: 0, end: 1},
		{src: `"abc`, start: 0, end: 1},
		{src: " \"abc", start: 1, end: 2},
		{src: "\"\\", start: 1, end: 2},
		{src: "\"abc\\", start: 4, end: 5},
		{src: `"\x00"`, start: 2, end: 3},
		{src: `"\a"`, start: 2, end: 3},
		{src: `"\v"`, start: 2, end: 3},
		{src: `"\0"`, start: 2, end: 3},
		{src: `"\U00000000"`, start: 2, end: 3},
		{src: `"\'"`, start: 2, end: 3},
		{src: `"\u`, start: 3, end: 3},
		{src: `"\u"`, start: 3, end: 4},
		{src: `"\u1`, start: 4, end: 4},
		{src: `"\u12`, start: 5, end: 5},
		{src: `"\u123`, start: 6, end: 6},
		{src: `"\u123x"`, start: 6, end: 7},
		{src: `"\u000g"`, start: 6, end: 7},
		{src: `"\u1234`, start: 0, end: 1},
		{src: `"\u１２３４"`, start: 3, end: 6},
		{src: "/*", start: 0, end: 2},
		{src: "/*x", start: 0, end: 2},
		{src: "/*x*", start: 0, end: 2},
		{src: " /*", start: 1, end: 3},
		{src: "/**", start: 0, end: 2},
		{src: "/*/", start: 0, end: 2},
		{src: "/**/ /*", start: 5, end: 7},
		{src: "//x\n/*", start: 4, end: 6},
		{src: "/*\xff*/\xff", start: 5, end: 6},
	} {
		t.Run(tc.src, func(t *testing.T) {
			err := scanError(t, tc.src)
			if want := token.NewSpan(tc.start, tc.end); err.Span != want {
				t.Errorf("error span = %v, want %v; error: %v", err.Span, want, err)
			}
		})
	}
	for c := byte(0); c < 0x20; c++ {
		src := string([]byte{'"', c, '"'})
		err := scanError(t, src)
		if want := token.NewSpan(1, 2); err.Span != want {
			t.Errorf("control byte %x: span = %v, want %v", c, err.Span, want)
		}
	}
}

func TestLexerFixtures(t *testing.T) {
	t.Parallel()

	for _, pattern := range []string{
		"testdata/json/*.json",
		"../grammar/examples/*.ks",
	} {
		paths, err := filepath.Glob(pattern)
		if err != nil || len(paths) == 0 {
			t.Fatalf("glob %q: paths=%v, error=%v", pattern, paths, err)
		}
		for _, path := range paths {
			t.Run(filepath.Base(path), func(t *testing.T) {
				src, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if filepath.Ext(path) == ".json" && !json.Valid(src) {
					t.Fatal("fixture is not valid JSON")
				}
				checkLex(t, string(src))
			})
		}
	}
}

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

func checkLex(t testing.TB, src string) []lexeme {
	t.Helper()

	l := lexer{src: src}
	var result []lexeme
	for {
		before := l.off
		x, err := l.next()
		if err != nil {
			t.Fatalf("scan %q: %v", src, err)
		}
		checkLexeme(t, src, before, l.off, x)
		if x.kind == token.EOF {
			checkEOF(t, &l, x)
			return result
		}
		result = append(result, x)
	}
}

func checkLexeme(t testing.TB, src string, before, after int, x lexeme) {
	t.Helper()

	// NOTE(i4k): checkLexeme check invariants after a successful l.next().
	// If this fails, it means some importance invariance of the lexer changed and
	// you better be sure the new behavior is sound to avoid issues like lexer skipping runes
	// or computing invalid spans.
	start, end := x.span.Start.Offset(), x.span.End.Offset()
	if !x.span.Valid() || start < before || end > len(src) || after != end {
		t.Fatalf("invalid token %#v, cursor %d -> %d, input length %d", x, before, after, len(src))
	}
	if skipped := src[before:start]; strings.TrimFunc(skipped, unicode.IsSpace) != "" {
		t.Fatalf("scanner skipped non-whitespace %q before token %#v", skipped, x)
	}
	if x.kind == token.Illegal {
		t.Fatal("successful scan returned Illegal")
	}
	if x.kind == token.EOF {
		if x.span != token.NewSpan(len(src), len(src)) {
			t.Fatalf("EOF span = %v, input length %d", x.span, len(src))
		}
	} else if start == end || after <= before {
		t.Fatalf("scan made no progress: %#v, cursor %d -> %d", x, before, after)
	}
	if x.kind == token.Comment {
		raw := src[start:end]
		switch {
		case strings.HasPrefix(raw, "//"):
			if strings.Contains(raw, "\n") || end < len(src) && src[end] != '\n' {
				t.Fatalf("incorrect line comment span %v for %q", x.span, raw)
			}
		case strings.HasPrefix(raw, "/*"):
			if len(raw) < 4 || strings.Index(raw[2:], "*/") != len(raw)-4 {
				t.Fatalf("incorrect block comment span %v for %q", x.span, raw)
			}
		default:
			t.Fatalf("comment lacks its opening delimiter: %q", raw)
		}
	}
}

func checkEOF(t testing.TB, l *lexer, want lexeme) {
	t.Helper()

	for range 2 {
		got, err := l.next()
		if err != nil || got != want || l.off != len(l.src) {
			t.Fatalf("EOF is not stable: token=%#v, error=%v, offset=%d", got, err, l.off)
		}
	}
}

func scanError(t testing.TB, src string) *Error {
	t.Helper()

	l := lexer{src: src}
	for {
		before := l.off
		x, err := l.next()
		if err != nil {
			return checkLexError(t, src, err)
		}
		checkLexeme(t, src, before, l.off, x)
		if x.kind == token.EOF {
			t.Fatalf("scan %q succeeded, want error", src)
		}
	}
}

func checkLexError(t testing.TB, src string, err error) *Error {
	t.Helper()
	var syntax *Error
	if !errors.As(err, &syntax) {
		t.Fatalf("error has type %T, want *Error", err)
	}
	if syntax.Message == "" || !syntax.Span.Valid() || syntax.Span.End.Offset() > len(src) {
		t.Fatalf("invalid error %#v for input length %d", syntax, len(src))
	}
	return syntax
}
