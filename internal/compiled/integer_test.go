package compiled

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

var google = "1" + strings.Repeat("0", 100)

func compileRawInt(t *testing.T, a *Arena, raw string) TypeID {
	t.Helper()
	var z token.Span
	tree := ast.New()
	root := tree.AddInt(raw, z)
	id, err := Compile(a, tree, root)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCompileCanonicalizeInt(t *testing.T) {
	t.Parallel()

	a := NewArena()
	x := compileRawInt(t, a, google)
	y := compileRawInt(t, a, "+"+google)
	z := compileRawInt(t, a, "-"+google)
	zero := compileRawInt(t, a, "0")
	negzero := compileRawInt(t, a, "-0")

	if x != y {
		t.Fatalf("same exact big int not interned: %d != %d", x, y)
	}
	if x == z {
		t.Fatalf("different big int interned: %d == %d", x, z)
	}
	if zero != negzero {
		t.Fatalf("canonical zero was not interned: %d != %d", zero, negzero)
	}
	for _, id := range []TypeID{x, y, z, zero, negzero} {
		if got := a.Node(id).Kind(); got != IntLit {
			t.Fatalf("kind %s is different than %s", got, IntLit)
		}
	}
}

func TestIntegerLiteralFastPath(t *testing.T) {
	t.Parallel()

	// NOTE(i4k): here we test if small integer are interned as normal Go int64 integers.

	for _, val := range []string{
		strconv.FormatInt(math.MaxInt64, 10),
		strconv.FormatInt(math.MinInt64, 10),
		"+0",
		"-0",
		"+1337",
		"-1337",
	} {
		a := NewArena()
		id := compileRawInt(t, a, val)
		if data := a.Node(id).data; data <= 0 {
			t.Fatalf("unexpected data %d, expected positive value", data)
		}
		if got := len(a.bigInts); got != 1 {
			t.Fatalf("unexpected bigInt interning: %d", got)
		}
		if got := len(a.bigIntBytes); got != 0 {
			t.Fatalf("unexpected bigIntBytes usage: %d", got)
		}
		if got := len(a.ints); got != 2 {
			t.Fatalf("unexpected number of interned ints: %d", got)
		}
	}

	// NOTE(i4k): here we test if big integers use the slower path
	for _, val := range []string{
		strconv.FormatUint(math.MaxInt64+1, 10),
		"-" + strconv.FormatUint(math.MaxInt64+2, 10),
		google,
	} {
		t.Run("bigInt/"+val, func(t *testing.T) {
			a := NewArena()
			id := compileRawInt(t, a, val)
			if data := a.Node(id).data; data >= 0 {
				t.Fatalf("unexpected data %d, expected a negative value", data)
			}
			if got := len(a.bigInts); got != 2 {
				t.Fatalf("unexpected number of interned bigInts: %d, expected 2", got)
			}
			if got := len(a.bigIntBytes); got == 0 {
				t.Fatalf("unexpected number of interned bytes: %d, expected zero", got)
			}
		})
	}
}
