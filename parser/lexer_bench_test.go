package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/birdie-ai/katschema/parser/token"
)

func BenchmarkLex(b *testing.B) {
	inputs := append(parseBenchmarkInputs(),
		testcase{name: "Identifiers/long", src: strings.Repeat("identifier", 8192)},
		testcase{name: "Comments/mixed", src: strings.Repeat("/* block */ value // line\r\n", 8192)},
	)
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
