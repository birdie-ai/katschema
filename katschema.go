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

	// Path is a field path through a compiled value.
	//   Path{"accounts", "id"} means `accounts.id`.
	Path []string

	// Field represents one object field.
	Field struct {
		Name     string
		Type     Type
		Optional bool
		metadata Metadata
	}

	// Metadata is the compiled set of attributes attached to a type or field.
	Metadata struct {
		arena *compiled.Arena
		attrs []compiled.Attribute
	}

	// Compiler owns the canonical type used by a group of related compilations.
	// NOTE(i4k): Types compiled by different compiler instances **cannot** be used together!
	// Compiler is not safe for concurrent Compile calls; synchronize reuse or
	// give each concurrent request its own compiler.
	Compiler struct {
		arena    *compiled.Arena
		resolver TypeResolver
		resolved map[string]compiled.TypeID
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
	ErrValidation   = errors.New("katschema validation")
	ErrUnknownType  = compiled.ErrUnknownType
	ErrResolveCycle = compiled.ErrResolveCycle
)

// TypeResolver resolves names that are not Katschema builtins. Returning
// ErrUnknownType indicates that the name is not defined.
type TypeResolver interface {
	Resolve(name string) (TypeResolution, error)
}

// TypeResolution is the result of resolving a user-defined type. CacheKey
// identifies the definition's scope; it may include a tenant or request
// identity when the same logical name has different definitions.
type TypeResolution struct {
	Value    ks.Value
	CacheKey string
}

// ResolverFunc adapts a function to TypeResolver.
type ResolverFunc func(name string) (TypeResolution, error)

func (f ResolverFunc) Resolve(name string) (TypeResolution, error) { return f(name) }

func NewCompiler() *Compiler {
	return &Compiler{arena: compiled.NewArena()}
}

// TODO(i4k): introduce a CompilerOption because it seems we will need to configure a lot
// this compiler.

// NewCompilerWithResolver creates a compiler that expands user-defined names.
func NewCompilerWithResolver(resolver TypeResolver) *Compiler {
	return &Compiler{arena: compiled.NewArena(), resolver: resolver, resolved: make(map[string]compiled.TypeID)}
}

// Compile compiles a Katschema value into an immutable semantic type.
func Compile(value ks.Value) (Type, error) {
	return NewCompiler().Compile(value)
}

// CompileWithResolver compiles a value using a resolver for non-builtin names.
func CompileWithResolver(value ks.Value, resolver TypeResolver) (Type, error) {
	return NewCompilerWithResolver(resolver).Compile(value)
}

func (c *Compiler) Compile(value ks.Value) (Type, error) {
	tree, root, err := ks.Build(value)
	if err != nil {
		return Type{}, err
	}

	resolve := compiled.TypeResolver(nil)
	if c.resolver != nil {
		resolve = func(name string) (compiled.ResolvedType, error) {
			resolution, err := c.resolver.Resolve(name)
			if err != nil {
				return compiled.ResolvedType{}, err
			}
			tree, root, err := ks.Build(resolution.Value)
			return compiled.ResolvedType{Tree: tree, Root: root, CacheKey: resolution.CacheKey}, err
		}
	}
	id, err := compiled.CompileWithResolver(c.arena, tree, root, resolve, c.resolved)
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
			metadata: Metadata{arena: t.arena, attrs: field.Metadata()},
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
				metadata: Metadata{arena: t.arena, attrs: field.Metadata()},
			}, true
		}
	}
	return Field{}, false
}

// Metadata returns attributes attached directly to this field.
func (f Field) Metadata() Metadata { return f.metadata }

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

// Traverse returns the compiled type at path.
// The path has the same semantics as "dot traversal" in many languages but here decoded
// in the Path type (list of strings).
// It has the same common semantics for object but when traversing over lists it has a
// special behavior of unwrapping the list element type transparently.
// Example: if the object is:
//
//	{
//	  "a": [
//	    {"b": 1}
//	  ],
//	  "c": {
//	    "d": 1
//	  }
//	}
//
// Then `a.b` and `c.d` both return 1.
func (t Type) Traverse(path Path) (Type, bool) {
	for _, name := range path {
		for {
			base, ok := t.Base()
			if !ok {
				break
			}
			t = base
		}
		if element, ok := t.Element(); ok {
			// Element is defined only for schema lists. Literal arrays are
			// compiled as tuples and must not be traversed as collections.
			t = element
			for {
				base, ok := t.Base()
				if !ok {
					break
				}
				t = base
			}
		}
		field, ok := t.Field(name)
		if !ok {
			return Type{}, false
		}
		t = field.Type
	}
	return t, true
}

// Fingerprint returns the canonical identity of the compiled semantic type.
func (t Type) Fingerprint() uint64 {
	return t.arena.Fingerprint(t.id)
}

// SemanticFingerprint excludes metadata and identifies the accepted values.
func (t Type) SemanticFingerprint() uint64 {
	return t.arena.SemanticFingerprint(t.id)
}

// DefinitionFingerprint includes metadata and identifies the complete compiled definition.
func (t Type) DefinitionFingerprint() uint64 { return t.Fingerprint() }

// Metadata returns attributes attached directly to this type.
func (t Type) Metadata() Metadata {
	return Metadata{arena: t.arena, attrs: t.arena.TypeMetadata(t.id)}
}

// Get returns an attribute value. A flag attribute is returned as true.
func (m Metadata) Get(name string) (any, bool) {
	for _, attr := range m.attrs {
		if attr.NameValue(m.arena) != name {
			continue
		}
		if !attr.Has() {
			return true, true
		}
		value, err := m.arena.MetadataValue(attr.ValueID())
		return value, err == nil
	}
	return nil, false
}

func (m Metadata) Has(name string) bool {
	_, ok := m.Get(name)
	return ok
}

// SubtypeOf reports whether every value accepted by t is accepted by other.
func (t Type) SubtypeOf(other Type) bool {
	return t.arena.Subtype(t.id, other.id)
}

// Overlay returns the effective object type formed by placing top over t.
// Both types must belong to the same compiler and be objects.
func (t Type) Overlay(top Type) (Type, error) {
	if t.arena != top.arena {
		return Type{}, fmt.Errorf("overlay types belong to different compilers")
	}
	id, err := t.arena.Overlay(t.id, top.id)
	if err != nil {
		return Type{}, err
	}
	return Type{arena: t.arena, id: id}, nil
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
