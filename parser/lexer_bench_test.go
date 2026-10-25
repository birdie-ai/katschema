package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/birdie-ai/katschema/parser/token"
)

func BenchmarkLex(b *testing.B) {
	type testcase struct {
		name string
		src  string
	}
	inputs := []testcase{
		{name: "JSON/small", src: `{"id":1337,"name":"Richard Feynman","active":true,"tags":["physics","electronics"]}`},
		{name: "JSON/large", src: "[" + strings.Repeat(`{"id":137,"name":"Richard Feynman","active":true,"tags":["physics","electronics"]},`, 2047) + `null]`},
		{name: "Strings/escapes", src: `"` + strings.Repeat(`\"\\\/\b\f\n\r\t\u0067\uD834\uDD1E`, 1024) + `"`},
		{name: "Identifiers/long", src: strings.Repeat("identifier", 8192)},
		{name: "Numbers/long", src: strings.Repeat("9", 8192) + "." + strings.Repeat("9", 8192) + "e+" + strings.Repeat("9", 8192)},
		{name: "Comments/long", src: "/*" + strings.Repeat(" a comment line\n", 8192) + "*/"},
		{name: "Comments/mixed", src: strings.Repeat("/* block */ value // line\r\n", 8192)},
	}
	for _, size := range []int{1024, 2048, 4096} {
		var src strings.Builder
		src.WriteByte('{')
		for i := range size {
			if i > 0 {
				src.WriteByte(',')
			}
			fmt.Fprintf(&src, `"field%04d":%d`, i, i)
		}
		src.WriteByte('}')
		inputs = append(inputs, testcase{name: fmt.Sprintf("JSON/wide_%d", size), src: src.String()})
	}
	for _, size := range []int{1024, 2048, 4096} {
		inputs = append(inputs, testcase{
			name: fmt.Sprintf("JSON/scale_%dx", size/1024), src: "[" + strings.Repeat("1,", size-1) + "1]",
		})
	}
	paths, err := filepath.Glob("../grammar/examples/*.ks")
	if err != nil || len(paths) == 0 {
		b.Fatalf("example fixtures: paths=%v, error=%v", paths, err)
	}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		inputs = append(inputs, testcase{name: "Katschema/" + filepath.Base(path), src: string(src)})
	}
	for _, input := range inputs {
		b.Run(input.name, func(b *testing.B) {
			want := len(checkLex(b, input.src))
			b.ReportAllocs()
			b.SetBytes(int64(len(input.src)))
			for b.Loop() {
				l := lexer{src: input.src}
				count := 0
				for {
					x, err := l.next()
					if err != nil {
						b.Fatal(err)
					}
					if x.kind == token.EOF {
						break
					}
					count++
				}
				if count != want {
					b.Fatalf("token count = %d, want %d", count, want)
				}
			}
		})
	}
}
