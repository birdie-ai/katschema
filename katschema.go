package katschema

import (
	"context"
	"errors"
	"fmt"

	"github.com/birdie-ai/katschema/internal/compiled"
	"github.com/birdie-ai/katschema/ks"
)

type (
	// Kind represents the semantic kind of a compiled type.
	Kind = compiled.Kind

	// Type is an immutable, read-only view of a compiled schema type.
	// Type values are safe to copy and can refer to different compiler arenas.
	Type struct {
		ref compiled.Type
	}

	// Path is a field path through a compiled value.
	// Path{"accounts", "id"} means accounts.id.
	Path []string

	// Field represents one object field.
	Field struct {
		Name     string
		Type     Type
		Optional bool
		metadata Metadata
	}

	// Metadata is the compiled set of attributes attached to a type or field.
	Metadata = compiled.Metadata

	// Compiler owns the canonical type arena used by a group of related
	// compilations.
	// NOTE(i4k): Compiler is not safe for concurrent Compile calls.
	// Synchronize reuse or give each concurrent request its own compiler.
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
	Resolve(ctx context.Context, name string) (TypeResolution, error)
}

// TypeResolution is the result of resolving a user-defined type. Name is an
// optional name for the resolved type. CacheKey identifies the definition's
// scope; it may include a tenant or request identity when the same logical
// name has different definitions.
type TypeResolution struct {
	Value    ks.Value
	Name     string
	CacheKey string
}

// ResolverFunc adapts a function to TypeResolver.
type ResolverFunc func(context.Context, string) (TypeResolution, error)

func (f ResolverFunc) Resolve(ctx context.Context, name string) (TypeResolution, error) {
	return f(ctx, name)
}

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
func Compile(ctx context.Context, value ks.Value) (Type, error) {
	return NewCompiler().Compile(ctx, value)
}

func CompileWithResolver(ctx context.Context, value ks.Value, resolver TypeResolver) (Type, error) {
	return NewCompilerWithResolver(resolver).Compile(ctx, value)
}

func (c *Compiler) Compile(ctx context.Context, value ks.Value) (Type, error) {
	tree, root, err := ks.Build(value)
	if err != nil {
		return Type{}, err
	}

	resolve := compiled.TypeResolver(nil)
	if c.resolver != nil {
		resolve = func(name string) (compiled.ResolvedType, error) {
			resolution, err := c.resolver.Resolve(ctx, name)
			if err != nil {
				return compiled.ResolvedType{}, err
			}
			tree, root, err := ks.Build(resolution.Value)
			return compiled.ResolvedType{Tree: tree, Root: root, Name: resolution.Name, CacheKey: resolution.CacheKey}, err
		}
	}
	id, err := compiled.CompileWithResolver(c.arena, tree, root, resolve, c.resolved)
	if err != nil {
		return Type{}, err
	}
	return Type{ref: c.arena.Type(id)}, nil
}

// Kind returns the semantic kind of the type.
func (t Type) Kind() Kind { return t.ref.Kind() }

// Name returns the optional name attached to a refined type. Unnamed
// refinements and non-refined types return the empty string.
func (t Type) Name() string { return t.ref.Name() }

// Element returns the element type of a list.
func (t Type) Element() (Type, bool) {
	element := t.ref.Element()
	return Type{ref: element}, element.Kind() != Invalid
}

// Len returns the number of tuple elements or object fields, or zero for other types.
func (t Type) Len() int { return t.ref.Len() }

// At returns the tuple element at index i, in source order.
// It panics for non-tuples and when index is outside [0, Len()).
func (t Type) At(i int) Type {
	element := t.ref.At(i)
	return Type{ref: element}
}

// Fields returns the fields of an object in canonical order.
func (t Type) Fields() []Field {
	fields := t.ref.Fields()
	if fields.Len() == 0 {
		return nil
	}
	out := make([]Field, fields.Len())
	for i := range out {
		out[i] = wrapField(fields.At(i))
	}
	return out
}

// Field returns an object field by name.
func (t Type) Field(name string) (Field, bool) {
	field, ok := t.ref.Fields().Get(name)
	if !ok {
		return Field{}, false
	}
	return wrapField(field), true
}

func wrapField(field compiled.FieldView) Field {
	return Field{
		Name:     field.Name(),
		Type:     Type{ref: field.Type()},
		Optional: field.Optional(),
		metadata: field.Metadata(),
	}
}

// Metadata returns attributes attached directly to this field.
func (f Field) Metadata() Metadata { return f.metadata }

// Variants returns the members of a sum type.
func (t Type) Variants() []Type {
	refs := t.ref.Variants()
	if len(refs) == 0 {
		return nil
	}
	out := make([]Type, len(refs))
	for i, ref := range refs {
		out[i] = Type{ref: ref}
	}
	return out
}

// Base returns the base type of a refinement.
func (t Type) Base() (Type, bool) {
	base := t.ref.Base()
	return Type{ref: base}, base.Kind() != Invalid
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
	ref, ok := t.ref.Traverse(path)
	if !ok {
		return Type{}, false
	}
	return Type{ref: ref}, true
}

// Fingerprint identifies the complete definition, including metadata and names.
// A matching hash is not a proof of semantic equality; use Equivalent for that.
func (t Type) Fingerprint() uint64 { return t.ref.Fingerprint() }

// SemanticFingerprint excludes metadata and names. Hash matches still require
// semantic comparison if equality must be established.
func (t Type) SemanticFingerprint() uint64 { return t.ref.SemanticFingerprint() }

// DefinitionFingerprint includes metadata and identifies the complete compiled definition.
func (t Type) DefinitionFingerprint() uint64 { return t.Fingerprint() }

// Metadata returns attributes attached directly to this type.
func (t Type) Metadata() Metadata { return t.ref.Metadata() }

// SubtypeOf reports whether every value accepted by t is accepted by other.
// Types are read in their original arenas; neither compiler is modified.
func (t Type) SubtypeOf(other Type) bool { return t.ref.SubtypeOf(other.ref) }

// Equivalent reports whether t and other accept the same values. Names and
// metadata do not affect equivalence. Types can belong to different compilers.
func (t Type) Equivalent(other Type) bool { return t.ref.Equivalent(other.ref) }

// Overlay returns a new type with top's object fields overlaid on bottom.
// Neither bottom, top, nor their arenas are modified. Fields from top replace
// complete field definitions, including optionality and metadata.
func Overlay(bottom, top Type) (Type, error) {
	ref, err := compiled.Overlay(bottom.ref, top.ref)
	if err != nil {
		return Type{}, err
	}
	return Type{ref: ref}, nil
}

// OverlayAt returns a new type with top's object fields overlaid on the object
// at path in bottom, preserving the enclosing types and field definitions.
func OverlayAt(bottom Type, path Path, top Type) (Type, error) {
	ref, err := compiled.OverlayAt(bottom.ref, path, top.ref)
	if err != nil {
		return Type{}, err
	}
	return Type{ref: ref}, nil
}

// Validate reports whether value is accepted by the compiled type.
func (t Type) Validate(value ks.Value) error {
	tree, root, err := ks.Build(value)
	if err != nil {
		return err
	}
	if !t.ref.Valid(tree, root) {
		return fmt.Errorf("%w: value does not conform to %s", ErrValidation, t.Kind())
	}
	return nil
}
