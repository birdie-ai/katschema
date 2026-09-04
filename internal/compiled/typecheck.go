package compiled

import "fmt"

// TypeCheck checks value against schema and returns value interpreted in the
// schema's type context.
func (a *Arena) TypeCheck(schema, value TypeID) (TypeID, error) {
	if !a.Subtype(value, schema) {
		return 0, fmt.Errorf("compiled: value %d is not a subtype of schema %d", value, schema)
	}
	return a.typeCheck(schema, value), nil
}

func (a *Arena) typeCheck(schema, value TypeID) TypeID {
	if schema == a.any || schema == value || value == a.never {
		return value
	}

	if sn := a.Node(schema); sn.kind == Sum {
		var selected TypeID
		for _, member := range a.sum(schema) {
			if !a.Subtype(value, member) {
				continue
			}
			if selected != 0 {
				// NOTE(i4k): There is no unique type context when multiple sum members
				// accept the same value. Keep the source interpretation.
				return value
			}
			selected = member
		}
		if selected != 0 {
			return a.typeCheck(selected, value)
		}
		return value
	}

	if atom, ok := a.Literal(value); ok {
		if schemaAtom, ok := a.Literal(schema); ok && a.compareAtom(atom, schemaAtom) == 0 {
			return schema
		}
		base := a.typeCheckBase(schema)
		atom = a.typeCheckAtom(base, atom)
		return a.internLiteral(base, atom)
	}

	sn, vn := a.Node(schema), a.Node(value)
	switch sn.kind {
	case List:
		elemSchema := TypeID(sn.data)
		switch vn.kind {
		case List:
			return a.internList(a.typeCheck(elemSchema, TypeID(vn.data)))
		case Tuple:
			elems := a.tuple(value)
			out := make([]TypeID, len(elems))
			for i, elem := range elems {
				out[i] = a.typeCheck(elemSchema, elem)
			}
			return a.internTuple(out)
		}
	case Tuple:
		if vn.kind == Tuple {
			schemaElems, valueElems := a.tuple(schema), a.tuple(value)
			out := make([]TypeID, len(valueElems))
			for i, elem := range valueElems {
				out[i] = a.typeCheck(schemaElems[i], elem)
			}
			return a.internTuple(out)
		}
	case Object:
		if vn.kind == Object {
			return a.typeCheckObject(schema, value)
		}
	}

	return value
}

func (a *Arena) typeCheckBase(schema TypeID) TypeID {
	n := a.Node(schema)
	if n.kind != Refined {
		return schema
	}

	r := a.refinements[n.data]
	d := a.constraints[r.constraint]
	if d.flags&constraintFloatConv != 0 {
		switch d.format {
		case f32Fmt:
			return a.f32
		case f64Fmt:
			return a.f64
		}
	}
	return r.base
}

func (a *Arena) typeCheckAtom(base, atom TypeID) TypeID {
	switch a.baseKind(base) {
	case Real:
		if a.Node(atom).kind == IntAtom {
			return a.realFromIntAtom(atom)
		}
	}
	return atom
}

func (a *Arena) typeCheckObject(schema, value TypeID) TypeID {
	schemaFields, valueFields := a.objectFields(schema), a.objectFields(value)
	out := make([]Field, len(valueFields))
	for i, field := range valueFields {
		j := a.findField(schemaFields, a.StringValue(field.Name))
		if j < 0 {
			return value
		}
		field.Value = a.typeCheck(schemaFields[j].Value, field.Value)
		out[i] = field
	}
	return a.internObject(out)
}
