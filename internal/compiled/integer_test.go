package compiled

import (
	"math"
	"math/big"
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
		atom, ok := a.Literal(id)
		if !ok || a.Node(atom).Kind() != IntAtom {
			t.Fatalf("compiled literal %d is not an integer literal", id)
		}
	}
}

func TestIntegerLiteralFastPath(t *testing.T) {
	t.Parallel()

	// NOTE(i4k): here we test if small integer are interned as normal Go int64 integers.

	{
		a := NewArena()
		before := len(a.ints)
		id := compileRawInt(t, a, strconv.FormatInt(math.MaxInt64, 10))
		after := len(a.ints)
		if before != after {
			// NOTE(i4k): this is an implementation detail. At the moment we are interning bounded
			// integer types during arena initialization, so it means interning integer edge values
			// is memoized.
			t.Fatal("unexpected -- okay to remove this test if the arena interning logic changed")
		}
		_, ok := a.Literal(id)
		if !ok {
			t.Fatalf("compiled literal %d is not a literal", id)
		}
	}

	for _, val := range []string{
		"10",
		"67",
		"+1337",
		"-1337",
	} {
		a := NewArena()
		id := compileRawInt(t, a, val)
		atom, ok := a.Literal(id)
		if !ok {
			t.Fatalf("compiled literal %d is not a singleton", id)
		}
		if data := a.Node(atom).data; data <= 0 {
			t.Fatalf("unexpected data %d, expected positive value", data)
		}
		// NOTE(i4k): this test depends on the number of interned stuff in the arena.init()
		// I just had to bump got=2 because we are interning "int64" max int, which is a bigint.
		if got := len(a.bigInts); got != 2 {
			t.Fatalf("unexpected bigInt interning: %d", got)
		}
		if got := len(a.bigIntBytes); got != 8 {
			t.Fatalf("unexpected bigIntBytes usage: %d", got)
		}
		if got := len(a.ints); got != 14 {
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
			atom, ok := a.Literal(id)
			if !ok {
				t.Fatalf("compiled literal %d is not a singleton", id)
			}
			if data := a.Node(atom).data; data >= 0 {
				t.Fatalf("unexpected data %d, expected a negative value", data)
			}
			if got := len(a.bigInts); got != 3 {
				t.Fatalf("unexpected number of interned bigInts: %d, expected 2", got)
			}
			if got := len(a.bigIntBytes); got == 0 {
				t.Fatalf("unexpected number of interned bytes: %d, expected zero", got)
			}
		})
	}
}

func TestBigIntCompareInts(t *testing.T) {
	t.Parallel()

	// NOTE(i4k): This is a sanity check in case we change the bigIntBytes implementation.
	// in order to compareInts() work, the bytes must be stored in an efficient comparable way.
	// Here we have a basic test checking if the implementation matches the [big.Int.Cmp] method.
	var x big.Int
	_, ok := x.SetString(google, 10)
	if !ok {
		t.Fatal("faile to create big.Int")
	}

	y := big.NewInt(1)
	var z big.Int
	z.Add(&x, y)

	a := NewArena()
	xx := compileRawInt(t, a, google)
	zz := compileRawInt(t, a, z.String())
	xxAtom, ok := a.Literal(xx)
	if !ok {
		t.Fatalf("compiled literal %d is not a singleton", xx)
	}
	zzAtom, ok := a.Literal(zz)
	if !ok {
		t.Fatalf("compiled literal %d is not a singleton", zz)
	}
	if got := a.compareInts(a.Node(xxAtom).data, a.Node(zzAtom).data); got != x.Cmp(&z) {
		t.Fatalf("compareInts(x, z) != x.Cmp(z): %d != %d", got, x.Cmp(&z))
	}
	if got := a.compareInts(a.Node(zzAtom).data, a.Node(xxAtom).data); got != z.Cmp(&x) {
		t.Fatalf("compareInts(z, x) != z.Cmp(x): %d != %d", got, z.Cmp(&x))
	}
}
