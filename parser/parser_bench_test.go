package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/birdie-ai/katschema/parser/ast"
)

type testcase struct {
	name string
	src  string
	json bool
}

func parseBenchmarkInputs() []testcase {
	inputs := []testcase{
		{name: "JSON/small", src: `{"id":1337,"name":"Richard Feynman","active":true,"tags":["physics","electronics"]}`, json: true},
		{name: "JSON/large", src: "[" + strings.Repeat(`{"id":137,"name":"Richard Feynman","active":true,"tags":["physics","electronics"]},`, 2047) + `null]`, json: true},
		{name: "JSON/escapes", src: `"` + strings.Repeat(`\"\\\/\b\f\n\r\t\u0067\uD834\uDD1E`, 1024) + `"`, json: true},
		{name: "JSON/long_number", src: strings.Repeat("9", 8192) + "." + strings.Repeat("9", 8192) + "e+" + strings.Repeat("9", 8192), json: true},
		{name: "Schema/basic", src: `{"name":(string),"age":(int)}`},
		{name: "Schema/structural", src: `({"name":(string),"address":({"city":(string),"country":(string)}),"tags":[(string)]})`},
		{name: "Comments/long", src: "/*" + strings.Repeat(" a comment line\n", 8192) + "*/null"},
		{name: "Comments/many", src: "[" + strings.Repeat("/* item */1,// line\n", 4095) + "/* last */1]"},
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
		inputs = append(inputs, testcase{name: fmt.Sprintf("JSON/wide_%d", size), src: src.String(), json: true})
		inputs = append(inputs, testcase{name: fmt.Sprintf("JSON/scale_%dx", size/1024), src: "[" + strings.Repeat("1,", size-1) + "1]", json: true})
	}
	for _, depth := range []int{16, 64, 256} {
		inputs = append(inputs, testcase{
			name: fmt.Sprintf("JSON/depth_%d", depth),
			src:  strings.Repeat("[", depth) + "null" + strings.Repeat("]", depth),
			json: true,
		})
	}
	return inputs
}

func BenchmarkParse(b *testing.B) {
	for _, input := range parseBenchmarkInputs() {
		b.Run(input.name, func(b *testing.B) {
			if input.json && !json.Valid([]byte(input.src)) {
				b.Fatal("invalid JSON fixture")
			}
			tree, root, err := Parse(input.src)
			if err != nil {
				b.Fatal(err)
			}
			wantKind := tree.Node(root).Kind()
			wantComments := len(tree.Comments())
			b.ReportAllocs()
			b.SetBytes(int64(len(input.src)))
			for b.Loop() {
				tree, root, err = Parse(input.src)
				if err != nil {
					b.Fatal(err)
				}
			}
			if root == 0 || wantKind == ast.Invalid || tree.Node(root).Kind() != wantKind || len(tree.Comments()) != wantComments {
				b.Fatal("inconsistent parse result")
			}
		})
	}
}

func BenchmarkJSONDecode(b *testing.B) {
	for _, input := range parseBenchmarkInputs() {
		if !input.json {
			continue
		}
		b.Run(input.name, func(b *testing.B) {
			if !json.Valid([]byte(input.src)) {
				b.Fatal("invalid JSON fixture")
			}
			var value any
			b.ReportAllocs()
			b.SetBytes(int64(len(input.src)))
			for b.Loop() {
				value = nil
				decoder := json.NewDecoder(strings.NewReader(input.src))
				decoder.UseNumber()
				if err := decoder.Decode(&value); err != nil {
					b.Fatal(err)
				}
				var trailing any
				if err := decoder.Decode(&trailing); err != io.EOF {
					b.Fatalf("expected EOF, got %v", err)
				}
			}
			if value == nil {
				b.Fatal("missing decoded value")
			}
		})
	}
}
