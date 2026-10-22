package compiled

import (
	"fmt"
	"sort"
)

type metadata struct {
	off         int32
	len         int32
	fingerprint uint64
}

// Metadata is a read-only reference to the attributes owned by an arena.
type Metadata struct {
	a  *Arena
	id MetadataID
}

func (m Metadata) Get(name string) (any, bool) {
	for _, attr := range m.a.metadataAttrsFor(m.id) {
		if m.a.StringValue(attr.Name) != name {
			continue
		}
		if !attr.HasValue {
			return true, true
		}
		value, err := m.a.MetadataValue(attr.Value)
		return value, err == nil
	}
	return nil, false
}

func (m Metadata) Has(name string) bool {
	_, ok := m.Get(name)
	return ok
}

type Attribute struct {
	Name     StringID
	Value    TypeID
	HasValue bool
}

func (a *Arena) internMetadata(attrs []Attribute) MetadataID {
	if len(attrs) == 0 {
		return 0
	}
	// NOTE(i4k): attrs is built by the compiler and is not shared with the arena, so it
	// can be canonicalized in place. We are not copying them here as an optimization!
	// Have this in mind if individual attr are shared later!
	sort.Slice(attrs, func(i, j int) bool {
		return a.StringValue(attrs[i].Name) < a.StringValue(attrs[j].Name)
	})
	for i := 1; i < len(attrs); i++ {
		if attrs[i-1].Name == attrs[i].Name {
			panic("duplicate metadata attribute")
		}
	}

	fp := a.metadataFingerprintAttrs(attrs)
	for id := MetadataID(1); int(id) < len(a.metadata); id++ {
		if a.metadataFingerprint(id) == fp && equalAttributes(a.metadataAttrsFor(id), attrs) {
			return id
		}
	}
	id := MetadataID(len(a.metadata))
	a.metadata = append(a.metadata, metadata{off: int32(len(a.metadataAttrs)), len: int32(len(attrs)), fingerprint: fp})
	a.metadataAttrs = append(a.metadataAttrs, attrs...)
	return id
}

// mergeMetadata combines metadata from a resolved type with metadata declared
// on the reference that uses it. Attributes on the reference take precedence.
func (a *Arena) mergeMetadata(base, overlay MetadataID) MetadataID {
	if base == 0 {
		return overlay
	}
	if overlay == 0 {
		return base
	}

	merged := append([]Attribute(nil), a.metadataAttrsFor(base)...)
	for _, attr := range a.metadataAttrsFor(overlay) {
		found := false
		for i := range merged {
			if merged[i].Name == attr.Name {
				merged[i] = attr
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, attr)
		}
	}
	return a.internMetadata(merged)
}

func (a *Arena) metadataAttrsFor(id MetadataID) []Attribute {
	if id <= 0 || int(id) >= len(a.metadata) {
		return nil
	}
	r := a.metadata[id]
	return a.metadataAttrs[r.off : r.off+r.len]
}

func equalAttributes(a, b []Attribute) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (a *Arena) metadataFingerprintAttrs(attrs []Attribute) uint64 {
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(0xfe))
	for _, attr := range attrs {
		a.scratch = putstr(a.scratch, a.StringValue(attr.Name))
		a.scratch = append(a.scratch, boolByte(attr.HasValue))
		if attr.HasValue {
			a.scratch = putu64(a.scratch, a.Fingerprint(attr.Value))
		}
	}
	return a.hash(a.scratch)
}

func (a *Arena) metadataFingerprint(id MetadataID) uint64 {
	if id == 0 {
		// id=0 means no metadata
		return 0
	}
	return a.metadata[id].fingerprint
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

// SemanticFingerprint returns an identity that excludes metadata while still
// including structure, optionality, and constraints.
func (a *Arena) SemanticFingerprint(id TypeID) uint64 {
	if id <= 0 || int(id) >= len(a.semanticFingerprints) {
		return 0
	}
	return a.semanticFingerprints[id]
}

func (a *Arena) MetadataValue(id TypeID) (any, error) {
	if id == 0 {
		return nil, fmt.Errorf("invalid metadata value")
	}
	return a.value(id)
}

func (a *Arena) value(id TypeID) (any, error) {
	if atom, ok := a.Literal(id); ok {
		n := a.Node(atom)
		switch n.kind {
		case Null:
			return nil, nil
		case BoolAtom:
			return n.data != 0, nil
		case IntAtom:
			return a.int64(n.data), nil
		case RealAtom:
			return a.realString(atom), nil
		case StringAtom:
			return a.StringValue(StringID(n.data)), nil
		default:
			panic(n.kind)
		}
	}
	n := a.Node(id)
	switch n.kind {
	case Tuple:
		out := make([]any, 0, len(a.tuple(id)))
		for _, child := range a.tuple(id) {
			v, err := a.value(child)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case Object:
		out := make(map[string]any)
		for _, f := range a.objectFields(id) {
			v, err := a.value(f.Value)
			if err != nil {
				return nil, err
			}
			out[a.StringValue(f.Name)] = v
		}
		return out, nil
	default:
		return nil, fmt.Errorf("metadata value is not a literal: %s", n.kind)
	}
}
