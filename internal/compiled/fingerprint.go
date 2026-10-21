package compiled

import "slices"

// Fingerprint identifies the complete definition, including names and metadata.
// Equality of fingerprints is only a hash match, never a proof of equivalence.
func (t Type) Fingerprint() uint64 {
	if t.view != nil {
		return t.view.fingerprint
	}
	return t.a.Fingerprint(t.id)
}

func (t Type) SemanticFingerprint() uint64 {
	if t.view != nil {
		return t.view.semantic
	}
	return t.a.SemanticFingerprint(t.id)
}

// computeFingerprint reads child fingerprints without allocating an arena or
// changing its scratch buffer. It is used once when a node/view is created.
func (t Type) computeFingerprint(semantic bool) uint64 {
	if semantic && t.Kind() == Refined && t.constraint().id == 0 {
		return t.Base().SemanticFingerprint()
	}
	var buffer [128]byte
	return t.a.hash(t.fingerprintEncoding(buffer[:0], semantic))
}

func (t Type) fingerprintEncoding(out []byte, semantic bool) []byte {
	kind := t.Kind()
	childHash := func(child Type) uint64 {
		if semantic {
			return child.SemanticFingerprint()
		}
		return child.Fingerprint()
	}
	out = append(out, encodingVersion, byte(kind))
	switch kind {
	case Any, Never, Null, Bool, Int, Real, String:
		// These types have no payload beyond their kind.
	case BoolAtom:
		out = append(out, byte(t.a.Node(t.id).data))
	case IntAtom:
		out = t.a.putIntAtom(out, t.id)
	case RealAtom:
		v, digits := t.a.realData(t.a.Node(t.id).data)
		out = append(out, boolByte(v.negative()))
		out = put64(out, v.exp)
		out = put32(out, int32(len(digits)))
		out = append(out, digits...)
	case StringAtom:
		out = putstr(out, t.a.StringValue(StringID(t.a.Node(t.id).data)))
	case List:
		out = putu64(out, childHash(t.Element()))
	case Tuple:
		out = put32(out, int32(t.Len()))
		for i := 0; i < t.Len(); i++ {
			out = putu64(out, childHash(t.At(i)))
		}
	case Object:
		fields := t.Fields()
		out = put32(out, int32(fields.Len()))
		hasMetadata := false
		if !semantic {
			for i := 0; i < fields.Len(); i++ {
				hasMetadata = hasMetadata || fields.At(i).f.Metadata != 0
			}
		}
		for i := 0; i < fields.Len(); i++ {
			field := fields.At(i)
			out = putstr(out, field.Name())
			out = append(out, byte(field.f.Flags))
			out = putu64(out, childHash(field.Type()))
			if hasMetadata {
				out = putu64(out, field.a.metadataFingerprint(field.f.Metadata))
			}
		}
	case Sum:
		// Members are a set. Arena-local ID order must never enter an
		// externally comparable fingerprint.
		members := t.a.sum(t.id)
		hashes := make([]uint64, len(members))
		for i, id := range members {
			hashes[i] = childHash(t.a.Type(id))
		}
		slices.Sort(hashes)
		out = put32(out, int32(len(hashes)))
		for _, hash := range hashes {
			out = putu64(out, hash)
		}
	case Refined:
		out = putu64(out, childHash(t.Base()))
		out = putu64(out, t.a.constraintFingerprint(t.constraint().id))
		if !semantic {
			out = putu64(out, t.a.metadataFingerprint(t.Metadata().id))
			if name := t.Name(); name != "" {
				out = putstr(out, name)
			}
		}
	default:
		panic(kind)
	}
	return out
}
