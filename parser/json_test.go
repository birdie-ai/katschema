package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	t.Parallel()
	type testcase struct {
		src string
	}
	for _, tc := range []testcase{
		{src: "null"}, {src: "true"}, {src: "false"}, {src: "0"}, {src: "-0"},
		{src: "1.200E+009"}, {src: "-1.200e-009"}, {src: "1e999999999999999999999"},
		{src: strings.Repeat("9", 4096)}, {src: "-" + strings.Repeat("9", 4096)},
		{src: "1." + strings.Repeat("0", 4096) + "1e-999999999999999999999"},
		{src: `""`}, {src: `"\"\\\/\b\f\n\r\t\u0000\u001f\u007f"`},
		{src: `"λ牦牛𝄞"`}, {src: `"\uD834\uDD1E"`}, {src: `"\ud800"`}, {src: `"\udc00"`},
		{src: `"\ud800\ud800\udc00\udc00"`}, {src: "\"\xff\xc0\xaf\""},
		{src: "[]"}, {src: "{}"}, {src: "[null,true,false,0,-0,1.2e3,\"text\",[],{}]"},
		{src: `{"a":1,"a":2,"\u0061":3,"":4}`},
		{src: `{"\u0000":"\u0001","\ud800":"\udc00"}`},
		{src: " \r\n\t{\"nested\":[{\"values\":[1,2,3]}]}\t\n"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			checkJSON(t, tc.src)
		})
	}
}

func TestParseJSONFixtures(t *testing.T) {
	t.Parallel()
	type testcase struct {
		pattern string
	}
	for _, tc := range []testcase{
		{pattern: "testdata/json/*.json"},
		{pattern: "testdata/JSONTestSuite/y_*.json"},
	} {
		paths, err := filepath.Glob(tc.pattern)
		if err != nil {
			t.Fatalf("fixtures %s: want %d, error = %v", tc.pattern, len(paths), err)
		}
		for _, path := range paths {
			t.Run(filepath.Base(path), func(t *testing.T) {
				src, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				checkJSON(t, string(src))
			})
		}
	}
}

func TestParseJSONImplementationCases(t *testing.T) {
	t.Parallel()
	rejected := map[string]bool{
		"i_string_UTF-16LE_with_BOM.json":         true,
		"i_string_utf16BE_no_BOM.json":            true,
		"i_string_utf16LE_no_BOM.json":            true,
		"i_structure_UTF-8_BOM_empty_object.json": true,
	}
	paths, err := filepath.Glob("testdata/JSONTestSuite/i_*.json")
	if err != nil {
		t.Fatalf("implementation fixtures: count = %d, want 35, error = %v", len(paths), err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if rejected[filepath.Base(path)] {
				parseFailure(t, string(src))
				if json.Valid(src) {
					t.Fatal("rejected encoding should not be valid")
				}
				return
			}
			checkJSON(t, string(src))
		})
	}
}

func checkJSON(t testing.TB, src string) {
	t.Helper()
	if !json.Valid([]byte(src)) {
		t.Fatalf("JSON rejected fixture %q", src)
	}
	tree, root := parseOK(t, src)
	checkParsePrint(t, tree, root)
}
