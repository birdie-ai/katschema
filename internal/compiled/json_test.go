package compiled

import (
	"encoding/json"
	"testing"

	"github.com/birdie-ai/katschema/parser"
)

func TestJSON(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		`null`,
		`true`,
		`0`,
		`-0.0`,
		`1`,
		`1.0`,
		`-123456789012345678901234567890`,
		`1.23456789012345678901234567890`,
		`1e9223372036854775807`,
		`1e-9223372036854775808`,
		`[1,1.0,1,[],{}]`,
		`{"z":[null,false,"\u0000\"\\"],"a":{}}`,
	} {
		t.Run(src, func(t *testing.T) {
			tree, root, err := parser.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			a := NewArena()
			id, err := Compile(a, tree, root)
			if err != nil {
				t.Fatal(err)
			}
			value := a.Type(id)
			size := a.Len()
			data, err := value.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(data) {
				t.Fatalf("invalid JSON: %s", data)
			}
			if a.Len() != size {
				t.Fatal("MarshalJSON changed the arena")
			}
			tree, root, err = parser.Parse(string(data))
			if err != nil {
				t.Fatal(err)
			}
			b := NewArena()
			id, err = Compile(b, tree, root)
			if err != nil {
				t.Fatal(err)
			}
			if value.Fingerprint() != b.Type(id).Fingerprint() {
				t.Fatalf("round trip changed the compiled value: %s", data)
			}
		})
	}
}
