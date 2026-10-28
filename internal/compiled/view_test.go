package compiled

import (
	"testing"
)

func TestEmptyTypeViewsRetainArena(t *testing.T) {
	t.Parallel()
	a := NewArena()
	scalar := a.Type(a.Int())
	for _, tc := range []struct {
		name string
		typ  Type
	}{
		{
			name: "non-list element",
			typ:  scalar.Element(),
		},
		{
			name: "non-refined base",
			typ:  scalar.Base(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.typ.a != a || tc.typ.Kind() != Invalid {
				t.Fatal("empty child view lost its owning arena")
			}
		})
	}
	if fields := scalar.Fields(); fields.a != a || fields.Len() != 0 {
		t.Fatal("empty fields view lost its owning arena")
	}
	metadata := scalar.Metadata()
	if metadata.a != a || metadata.Has("missing") {
		t.Fatal("empty metadata view lost its owning arena")
	}
	if scalar.constraint().a != a {
		t.Fatal("empty constraint view lost its owning arena")
	}
}

func TestTypeWithoutArenaPanics(t *testing.T) {
	t.Parallel()
	// A real node ID without its owning arena is an internal invariant breach,
	// not an absent child or an unsupported operation.
	typ := Type{id: 1}
	for _, tc := range []struct {
		name string
		call func()
	}{
		{
			name: "kind",
			call: func() { typ.Kind() },
		},
		{
			name: "name",
			call: func() { typ.Name() },
		},
		{
			name: "fingerprint",
			call: func() { typ.Fingerprint() },
		},
		{
			name: "semantic fingerprint",
			call: func() { typ.SemanticFingerprint() },
		},
		{
			name: "literal",
			call: func() { typ.literal() },
		},
		{
			name: "metadata",
			call: func() { (Metadata{id: 1}).Get("missing") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("missing arena did not panic")
				}
			}()
			tc.call()
		})
	}
}
