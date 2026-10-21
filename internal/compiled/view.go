package compiled

import (
	"fmt"
	"sort"
)

// Type is an immutable reference to a compiled type. Its children can belong
// to different arenas. Every instance has an owning arena, including empty child
// views represented by ID zero.
type Type struct {
	a    *Arena
	id   TypeID
	view *typeView
}

// typeView represents an overlay result.
type typeView struct {
	kind        Kind
	child       Type
	fields      []FieldView
	fingerprint uint64
	semantic    uint64
}

func (a *Arena) Type(id TypeID) Type { return Type{a: a, id: id} }

// ID is meaningful only together with this type's arena. Derived views have
// no arena-local ID and return zero.
func (t Type) ID() TypeID {
	if t.view != nil {
		return 0
	}
	return t.id
}

func (t Type) Kind() Kind {
	if t.view != nil {
		return t.view.kind
	}
	return t.a.Node(t.id).kind
}

func (t Type) Name() string {
	return t.a.TypeName(t.id)
}

func (t Type) Element() Type {
	if t.Kind() != List {
		return t.a.Type(0)
	}
	if t.view != nil {
		return t.view.child
	}
	return t.a.Type(TypeID(t.a.Node(t.id).data))
}

func (t Type) Len() int {
	switch t.Kind() {
	case Tuple:
		return len(t.a.tuple(t.id))
	case Object:
		return t.Fields().Len()
	}
	return 0
}

func (t Type) At(i int) Type {
	if t.Kind() != Tuple {
		return t.a.Type(0)
	}
	v := t.a.tuple(t.id)
	if i < 0 || i >= len(v) {
		return t.a.Type(0)
	}
	return t.a.Type(v[i])
}

func (t Type) Fields() Fields {
	if t.Kind() != Object {
		return Fields{a: t.a}
	}
	if t.view != nil {
		return Fields{a: t.a, views: t.view.fields}
	}
	return Fields{a: t.a, v: t.a.objectFields(t.id)}
}

func (t Type) MustGetField(name string) FieldView { return t.Fields().MustGet(name) }

func (t Type) Base() Type {
	if t.Kind() != Refined {
		return t.a.Type(0)
	}
	if t.view != nil {
		return t.view.child
	}
	return t.a.Type(t.a.refinements[t.a.Node(t.id).data].base)
}

// Variants returns the members of a sum type. It returns nil for non-sum types.
func (t Type) Variants() []Type {
	if t.Kind() != Sum {
		return nil
	}
	ids := t.a.sum(t.id)
	out := make([]Type, len(ids))
	for i, id := range ids {
		out[i] = t.a.Type(id)
	}
	return out
}

func (t Type) Metadata() Metadata {
	if t.Kind() != Refined {
		return Metadata{a: t.a}
	}
	r := t.a.refinements[t.a.Node(t.id).data]
	return Metadata{a: t.a, id: r.metadata}
}

func (t Type) constraint() constraintRef {
	if t.Kind() != Refined {
		return constraintRef{a: t.a}
	}
	return constraintRef{a: t.a, id: t.a.refinements[t.a.Node(t.id).data].constraint}
}

// Overlay returns a new type with top's object fields overlaid on bottom.
// Neither input nor its arena is modified. Fields from top replace
// complete field definitions, including optionality and metadata.
func Overlay(bottom, top Type) (Type, error) {
	return OverlayAt(bottom, nil, top)
}

// OverlayAt returns a new type with top's object fields overlaid on the object
// at path in bottom, without modifying either input or its arena.
func OverlayAt(bottom Type, path []string, top Type) (Type, error) {
	for top.Kind() == Refined || top.Kind() == List {
		if top.Kind() == Refined {
			if top.constraint().id != 0 {
				return Type{}, fmt.Errorf("overlay patch wrappers must not have constraints")
			}
			top = top.Base()
		} else {
			top = top.Element()
		}
	}
	if top.Kind() != Object {
		return Type{}, fmt.Errorf("overlay top must contain an object")
	}
	return bottom.overlayFields(path, top.Fields())
}

func (t Type) overlayFields(path []string, top Fields) (Type, error) {
	view := &typeView{kind: t.Kind()}
	switch t.Kind() {
	case List, Refined:
		child := t.Element()
		if t.Kind() == Refined {
			child = t.Base()
		}
		var err error
		view.child, err = child.overlayFields(path, top)
		if err != nil {
			return Type{}, err
		}
	case Object:
		base := t.Fields()
		if len(path) > 0 {
			field, ok := base.Get(path[0])
			if !ok {
				return Type{}, fmt.Errorf("overlay field %q not found", path[0])
			}
			child, err := field.Type().overlayFields(path[1:], top)
			if err != nil {
				return Type{}, err
			}
			view.fields = make([]FieldView, base.Len())
			for i := range view.fields {
				view.fields[i] = base.At(i)
				if view.fields[i].Name() == path[0] {
					view.fields[i].typ = &child
				}
			}
			break
		}
		view.fields = make([]FieldView, 0, base.Len()+top.Len())
		for i, j := 0, 0; i < base.Len() || j < top.Len(); {
			switch {
			case i == base.Len():
				view.fields = append(view.fields, top.At(j))
				j++
			case j == top.Len():
				view.fields = append(view.fields, base.At(i))
				i++
			case base.At(i).Name() < top.At(j).Name():
				view.fields = append(view.fields, base.At(i))
				i++
			case top.At(j).Name() < base.At(i).Name():
				view.fields = append(view.fields, top.At(j))
				j++
			default:
				view.fields = append(view.fields, top.At(j))
				i++
				j++
			}
		}
	default:
		return Type{}, fmt.Errorf("overlay base must contain an object")
	}
	result := Type{a: t.a, id: t.id, view: view}
	view.fingerprint = result.computeFingerprint(false)
	view.semantic = result.computeFingerprint(true)
	return result, nil
}

type Fields struct {
	a     *Arena
	v     []Field
	views []FieldView
}

func (f Fields) Len() int {
	if f.views != nil {
		return len(f.views)
	}
	return len(f.v)
}

func (f Fields) At(i int) FieldView {
	if i < 0 || i >= f.Len() {
		return FieldView{}
	}
	if f.views != nil {
		return f.views[i]
	}
	return FieldView{a: f.a, f: f.v[i]}
}

func (f Fields) Get(name string) (FieldView, bool) {
	i := sort.Search(f.Len(), func(i int) bool { return f.At(i).Name() >= name })
	if i == f.Len() || f.At(i).Name() != name {
		return FieldView{}, false
	}
	return f.At(i), true
}

func (f Fields) MustGet(name string) FieldView {
	field, ok := f.Get(name)
	if !ok {
		panic(fmt.Sprintf("field %q not found", name))
	}
	return field
}

type FieldView struct {
	a *Arena
	f Field
	// typ replaces only the field's type in a derived parent view. Its name,
	// optionality and field metadata still belong to the original arena.
	typ *Type
}

func (f FieldView) Name() string { return f.a.StringValue(f.f.Name) }
func (f FieldView) Type() Type {
	if f.typ != nil {
		return *f.typ
	}
	return f.a.Type(f.f.Value)
}
func (f FieldView) Optional() bool     { return f.f.Optional() }
func (f FieldView) Metadata() Metadata { return Metadata{a: f.a, id: f.f.Metadata} }
