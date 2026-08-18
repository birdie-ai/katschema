package compiled

// Type is a cheap read-only view over a TypeID.
type Type struct {
	a  *Arena
	id TypeID
}

func (a *Arena) Type(id TypeID) Type { return Type{a: a, id: id} }

func (t Type) ID() TypeID { return t.id }

func (t Type) Kind() Kind {
	if t.a == nil {
		return Invalid
	}
	return t.a.Node(t.id).Kind()
}

func (t Type) Element() TypeID {
	if t.a == nil {
		return 0
	}
	n := t.a.Node(t.id)
	if n.kind != List {
		return 0
	}
	return TypeID(n.data)
}

func (t Type) Len() int {
	if t.a == nil {
		return 0
	}
	switch t.Kind() {
	case Tuple:
		return len(t.a.tuple(t.id))
	case Object:
		return len(t.a.objectFields(t.id))
	}
	return 0
}

func (t Type) At(i int) TypeID {
	if t.a == nil || t.Kind() != Tuple {
		return 0
	}
	v := t.a.tuple(t.id)
	if i < 0 || i >= len(v) {
		return 0
	}
	return v[i]
}

func (t Type) Fields() Fields {
	if t.a == nil || t.Kind() != Object {
		return Fields{}
	}
	return Fields{a: t.a, v: t.a.objectFields(t.id)}
}

func (t Type) Base() TypeID {
	if t.a == nil {
		return 0
	}
	n := t.a.Node(t.id)
	if n.kind != Refined {
		return 0
	}
	return t.a.refinements[n.data].base
}

type Fields struct {
	a *Arena
	v []Field
}

func (f Fields) Len() int { return len(f.v) }

func (f Fields) At(i int) FieldView {
	if i < 0 || i >= len(f.v) {
		return FieldView{}
	}
	return FieldView{a: f.a, f: f.v[i]}
}

type FieldView struct {
	a *Arena
	f Field
}

func (f FieldView) Name() string {
	if f.a == nil {
		return ""
	}
	return f.a.StringValue(f.f.Name)
}

func (f FieldView) Type() TypeID   { return f.f.Type }
func (f FieldView) Optional() bool { return f.f.Optional() }
