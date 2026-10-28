package compiled

import (
	"math/big"

	"github.com/birdie-ai/katschema/math/decimal"
)

func (t Type) IsNull() bool {
	v, ok := t.literal()
	return ok && v.Kind() == Null
}

func (t Type) BoolValue() (bool, bool) {
	v, ok := t.literal()
	if !ok || v.Kind() != BoolAtom {
		return false, false
	}
	return v.a.Node(v.id).data != 0, true
}

func (t Type) StringValue() (string, bool) {
	v, ok := t.literal()
	if !ok || v.Kind() != StringAtom {
		return "", false
	}
	return v.a.StringValue(StringID(v.a.Node(v.id).data)), true
}

func (t Type) IntValue() (*big.Int, bool) {
	v, ok := t.literal()
	if !ok || v.Kind() != IntAtom {
		return nil, false
	}
	data := v.a.Node(v.id).data
	if data > 0 {
		return big.NewInt(v.a.int64(data)), true
	}
	return v.a.bigInt(data), true
}

func (t Type) DecimalValue() (decimal.Number, bool) {
	v, ok := t.literal()
	if !ok || v.Kind() != RealAtom {
		return decimal.Number{}, false
	}
	return v.a.DecimalValue(v.id)
}
