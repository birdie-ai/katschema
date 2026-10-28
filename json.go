package katschema

// MarshalJSON encodes the concrete value represented by t as JSON.
func (t Type) MarshalJSON() ([]byte, error) { return t.ref.MarshalJSON() }
