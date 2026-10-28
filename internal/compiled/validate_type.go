package compiled

func (typ Type) ValidType(value Type) bool {
	return value.isValue() && typ.acceptsValue(value)
}

func (value Type) isValue() bool {
	if _, ok := value.literal(); ok {
		return true
	}
	switch value.Kind() {
	case Refined:
		base := value.Base()
		return base.isValue() && value.constraint().accepts(base.literalBase())
	case Tuple:
		for i := 0; i < value.Len(); i++ {
			if !value.At(i).isValue() {
				return false
			}
		}
		return true
	case Object:
		fields := value.Fields()
		for i := 0; i < fields.Len(); i++ {
			field := fields.At(i)
			if field.Optional() || !field.Type().isValue() {
				return false
			}
		}
		return true
	}
	return false
}

func (value Type) literalBase() Type {
	for value.Kind() == Refined {
		if atom, ok := value.literal(); ok {
			return atom
		}
		value = value.Base()
	}
	return value
}

// acceptsValue requires a concrete value whose own constraints have been checked.
func (typ Type) acceptsValue(value Type) bool {
	value = value.literalBase()
	kind := value.Kind()
	switch typ.Kind() {
	case Any:
		return true
	case Invalid, Never:
		return false
	case Null:
		return kind == Null
	case Bool:
		return kind == BoolAtom
	case Int:
		return kind == IntAtom
	case Real:
		return kind == IntAtom || kind == RealAtom
	case String:
		return kind == StringAtom
	case BoolAtom, IntAtom, StringAtom:
		return kind == typ.Kind() && compareAtoms(value, typ) == 0
	case RealAtom:
		return (kind == IntAtom || kind == RealAtom) && compareAtoms(value, typ) == 0
	case Refined:
		return typ.Base().acceptsValue(value) && typ.constraint().accepts(value)
	case List:
		if kind != Tuple {
			return false
		}
		element := typ.Element()
		for i := 0; i < value.Len(); i++ {
			if !element.acceptsValue(value.At(i)) {
				return false
			}
		}
		return true
	case Tuple:
		if kind != Tuple || typ.Len() != value.Len() {
			return false
		}
		for i := 0; i < value.Len(); i++ {
			if !typ.At(i).acceptsValue(value.At(i)) {
				return false
			}
		}
		return true
	case Object:
		return kind == Object && typ.acceptsObject(value)
	case Sum:
		for _, id := range typ.a.sum(typ.id) {
			if typ.a.Type(id).acceptsValue(value) {
				return true
			}
		}
		return false
	default:
		panic(typ.Kind())
	}
}

func (typ Type) acceptsObject(value Type) bool {
	want, got := typ.Fields(), value.Fields()
	j := 0
	for i := 0; i < want.Len(); i++ {
		field := want.At(i)
		if j == got.Len() || field.Name() != got.At(j).Name() {
			if !field.Optional() {
				return false
			}
			continue
		}
		if !field.Type().acceptsValue(got.At(j).Type()) {
			return false
		}
		j++
	}
	return j == got.Len()
}
