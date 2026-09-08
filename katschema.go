package katschema

import (
	"errors"
	"fmt"

	"github.com/birdie-ai/katschema/internal/compiled"
	"github.com/birdie-ai/katschema/ks"
)

type (
	// Kind represents the semantic kind of a compiled type.
	Kind = compiled.Kind

	// Type is an immutable, read-only view of a compiled schema type.
	//
	// Type values are safe to copy.
	Type struct {
		arena *compiled.Arena
		id    compiled.TypeID
	}

	// Field represents one object field.
	Field struct {
		Name     string
		Type     Type
		Optional bool
	}

	// Compiler owns the canonical type used by a group of related compilations.
	// NOTE(i4k): Types compiled by different compiler instances **cannot** be used together!
	Compiler struct {
		arena *compiled.Arena
	}
)

const (
	Invalid = compiled.Invalid
	Any     = compiled.Any
	Never   = compiled.Never
	Null    = compiled.Null
	Bool    = compiled.Bool
	Int     = compiled.Int
	String  = compiled.String
	List    = compiled.List
	Tuple   = compiled.Tuple
	Object  = compiled.Object
	Sum     = compiled.Sum
	Refined = compiled.Refined
	Real    = compiled.Real
)

var (
	ErrValidation = errors.New("katschema validation")
)

func NewCompiler() *Compiler {
	return &Compiler{arena: compiled.NewArena()}
}

// Compile compiles a Katschema value into an immutable semantic type.
func Compile(value ks.Value) (Type, error) {
	return NewCompiler().Compile(value)
}

func (c *Compiler) Compile(value ks.Value) (Type, error) {
	tree, root, err := ks.Build(value)
	if err != nil {
		return Type{}, err
	}

	id, err := compiled.Compile(c.arena, tree, root)
	if err != nil {
		return Type{}, err
	}
	return Type{arena: c.arena, id: id}, nil
}

// Kind returns the semantic kind of the type.
func (t Type) Kind() Kind {
	return t.view().Kind()
}

// Element returns the element type of a list.
func (t Type) Element() (Type, bool) {
	id := t.view().Element()
	if id == 0 {
		return Type{}, false
	}
	return Type{arena: t.arena, id: id}, true
}

// Fields returns the fields of an object in canonical order.
func (t Type) Fields() []Field {
	fields := t.view().Fields()
	length := fields.Len()
	if length == 0 {
		return nil
	}
	out := make([]Field, 0, length)
	for i := 0; i < length; i++ {
		field := fields.At(i)
		out = append(out, Field{
			Name:     field.Name(),
			Type:     Type{arena: t.arena, id: field.Type()},
			Optional: field.Optional(),
		})
	}
	return out
}

// Field returns an object field by name.
func (t Type) Field(name string) (Field, bool) {
	fields := t.view().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.At(i)
		if field.Name() == name {
			return Field{
				Name:     field.Name(),
				Type:     Type{arena: t.arena, id: field.Type()},
				Optional: field.Optional(),
			}, true
		}
	}
	return Field{}, false
}

// Variants returns the members of a sum type.
func (t Type) Variants() []Type {
	ids := t.view().Variants()
	if len(ids) == 0 {
		return nil
	}
	out := make([]Type, 0, len(ids))
	for _, id := range ids {
		out = append(out, Type{arena: t.arena, id: id})
	}
	return out
}

// Base returns the base type of a refinement.
func (t Type) Base() (Type, bool) {
	id := t.view().Base()
	if id == 0 {
		return Type{}, false
	}
	return Type{arena: t.arena, id: id}, true
}

// Fingerprint returns the canonical identity of the compiled semantic type.
func (t Type) Fingerprint() uint64 {
	return t.arena.Fingerprint(t.id)
}

// SubtypeOf reports whether every value accepted by t is accepted by other.
func (t Type) SubtypeOf(other Type) bool {
	return t.arena.Subtype(t.id, other.id)
}

// Validate reports whether value is accepted by the the compiled type.
func (t Type) Validate(value ks.Value) error {
	tree, root, err := ks.Build(value)
	if err != nil {
		return err
	}
	if !t.arena.Valid(t.id, tree, root) {
		return fmt.Errorf("%w: value does not conform to %s", ErrValidation, t.Kind())
	}
	return nil
}

func (t Type) view() compiled.Type {
	return t.arena.Type(t.id)
}
