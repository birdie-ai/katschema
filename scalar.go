package katschema

import (
	"math/big"

	"github.com/birdie-ai/katschema/math/decimal"
)

func (t Type) IsNull() bool                         { return t.ref.IsNull() }
func (t Type) BoolValue() (bool, bool)              { return t.ref.BoolValue() }
func (t Type) StringValue() (string, bool)          { return t.ref.StringValue() }
func (t Type) IntValue() (*big.Int, bool)           { return t.ref.IntValue() }
func (t Type) DecimalValue() (decimal.Number, bool) { return t.ref.DecimalValue() }
