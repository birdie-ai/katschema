package compiled

import (
	"sort"
)

type hashFunc func([]byte) uint64

// Arena owns a canonical semantic DAG.
//
// NOTE(i4k): Mutation is intentionally not concurrency-safe.
type Arena struct {
	nodes        []Node
	fingerprints []uint64
	hashNext     []TypeID
	hashHead     map[uint64]TypeID

	strings     []string
	stringIndex map[string]StringID

	refs    []TypeID
	tuples  []range32
	objects []range32
	fields  []Field
	sums    []range32

	ints    []int64
	numbers []float64

	refinements []refinement

	constraints     []constraintData
	constraintHash  []uint64
	constraintNext  []ConstraintID
	constraintHead  map[uint64]ConstraintID
	constraintEnums []TypeID

	// NOTE(i4k): this is only used by tests to force a collision and check if we
	// are not overriding existing entries in such cases.
	hash hashFunc

	// NOTE(i4k): scratch is a reused buffer for temporary encoding of nodes.
	// After [Compile] returns, the capacity of this buffer is equal the binary
	// encoding of the largest node encoded by the arena. In other words, the
	// scratch backing array never shrinks, which could be a source of leak if
	// the same arena is used for compiling an arbitrary large number of nodes.
	// There's no Reset() of the arena (jsut yet) and we can shring it to a sane
	// minimum size when we need this. I hope I don't regret this comment!
	scratch []byte

	anyID    TypeID
	neverID  TypeID
	nullID   TypeID
	boolID   TypeID
	intID    TypeID
	floatID  TypeID
	stringID TypeID
}

func NewArena() *Arena {
	return newArena(nil)
}

func newArena(h hashFunc) *Arena {
	a := &Arena{hash: h}
	a.init()
	return a
}

func (a *Arena) init() {
	if a.hashHead != nil {
		return
	}
	if a.hash == nil {
		a.hash = sum64
	}

	a.nodes = append(a.nodes, Node{})
	a.fingerprints = append(a.fingerprints, 0)
	a.hashNext = append(a.hashNext, 0)
	a.hashHead = make(map[uint64]TypeID)

	a.strings = append(a.strings, "")
	a.stringIndex = make(map[string]StringID)

	a.ints = append(a.ints, 0)
	a.numbers = append(a.numbers, 0)
	a.refinements = append(a.refinements, refinement{})

	a.constraints = append(a.constraints, constraintData{})
	a.constraintHash = append(a.constraintHash, 0)
	a.constraintNext = append(a.constraintNext, 0)
	a.constraintHead = make(map[uint64]ConstraintID)

	a.anyID = a.internSimple(Any)
	a.neverID = a.internSimple(Never)
	a.nullID = a.internSimple(Null)
	a.boolID = a.internSimple(Bool)
	a.intID = a.internSimple(Int)
	a.floatID = a.internSimple(Float)
	a.stringID = a.internSimple(String)
}

func (a *Arena) Any() TypeID    { a.init(); return a.anyID }
func (a *Arena) Never() TypeID  { a.init(); return a.neverID }
func (a *Arena) Null() TypeID   { a.init(); return a.nullID }
func (a *Arena) Bool() TypeID   { a.init(); return a.boolID }
func (a *Arena) Int() TypeID    { a.init(); return a.intID }
func (a *Arena) Float() TypeID  { a.init(); return a.floatID }
func (a *Arena) String() TypeID { a.init(); return a.stringID }

func (a *Arena) Len() int { a.init(); return len(a.nodes) - 1 }

func (a *Arena) Node(id TypeID) Node {
	if id <= 0 || int(id) >= len(a.nodes) {
		return Node{}
	}
	return a.nodes[id]
}

func (a *Arena) Fingerprint(id TypeID) uint64 {
	if id <= 0 || int(id) >= len(a.fingerprints) {
		return 0
	}
	return a.fingerprints[id]
}

func (a *Arena) StringValue(id StringID) string {
	if id <= 0 || int(id) >= len(a.strings) {
		return ""
	}
	return a.strings[id]
}

func (a *Arena) internString(s string) StringID {
	if id := a.stringIndex[s]; id != 0 {
		return id
	}
	id := StringID(len(a.strings))
	a.strings = append(a.strings, s)
	a.stringIndex[s] = id
	return id
}

func (a *Arena) appendNode(n Node, fp uint64, next TypeID) TypeID {
	id := TypeID(len(a.nodes))
	a.nodes = append(a.nodes, n)
	a.fingerprints = append(a.fingerprints, fp)
	a.hashNext = append(a.hashNext, next)
	a.hashHead[fp] = id
	return id
}

func (a *Arena) find(fp uint64, equal func(TypeID) bool) TypeID {
	for id := a.hashHead[fp]; id != 0; id = a.hashNext[id] {
		if equal(id) {
			return id
		}
	}
	return 0
}

func (a *Arena) internSimple(k Kind) TypeID {
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(k))
	fp := a.hash(a.scratch)
	id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		return n.kind == k && n.data == 0
	})
	if id != 0 {
		return id
	}
	return a.appendNode(Node{kind: k}, fp, a.hashHead[fp])
}

func (a *Arena) internBool(v bool) TypeID {
	var x int32
	if v {
		x = 1
	}
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(BoolLit), byte(x))
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		return n.kind == BoolLit && n.data == x
	}); id != 0 {
		return id
	}
	return a.appendNode(Node{kind: BoolLit, data: x}, fp, a.hashHead[fp])
}

func (a *Arena) internInt(v int64) TypeID {
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(IntLit))
	a.scratch = put64(a.scratch, v)
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		return n.kind == IntLit && a.ints[n.data] == v
	}); id != 0 {
		return id
	}
	i := int32(len(a.ints))
	a.ints = append(a.ints, v)
	return a.appendNode(Node{kind: IntLit, data: i}, fp, a.hashHead[fp])
}

func (a *Arena) internFloat(v float64) TypeID {
	v = canonicalFloat(v)
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(FloatLit))
	a.scratch = putf64(a.scratch, v)
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		return n.kind == FloatLit && floatEqual(a.numbers[n.data], v)
	}); id != 0 {
		return id
	}
	i := int32(len(a.numbers))
	a.numbers = append(a.numbers, v)
	return a.appendNode(Node{kind: FloatLit, data: i}, fp, a.hashHead[fp])
}

func (a *Arena) internStringLit(s string) TypeID {
	name := a.internString(s)
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(StringLit))
	a.scratch = putstr(a.scratch, s)
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		return n.kind == StringLit && StringID(n.data) == name
	}); id != 0 {
		return id
	}
	return a.appendNode(Node{kind: StringLit, data: int32(name)}, fp, a.hashHead[fp])
}

func (a *Arena) internList(elem TypeID) TypeID {
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(List))
	a.scratch = putu64(a.scratch, a.Fingerprint(elem))
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		return n.kind == List && TypeID(n.data) == elem
	}); id != 0 {
		return id
	}
	return a.appendNode(Node{kind: List, data: int32(elem)}, fp, a.hashHead[fp])
}

func (a *Arena) internTuple(elems []TypeID) TypeID {
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(Tuple))
	a.scratch = put32(a.scratch, int32(len(elems)))
	for _, id := range elems {
		a.scratch = putu64(a.scratch, a.Fingerprint(id))
	}
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		if n.kind != Tuple {
			return false
		}
		old := a.tuple(TypeID(id))
		return equalTypeIDs(old, elems)
	}); id != 0 {
		return id
	}
	r := range32{off: int32(len(a.refs)), len: int32(len(elems))}
	a.refs = append(a.refs, elems...)
	i := int32(len(a.tuples))
	a.tuples = append(a.tuples, r)
	return a.appendNode(Node{kind: Tuple, data: i}, fp, a.hashHead[fp])
}

// Invariants of object interning.
// 1. Source order of fields is semantically irrelevant.
// 2. Once interned, fields are sorted by name lexicographically.
// 3. Fields are comparable using Go's native == or !=.
func (a *Arena) internObject(fields []Field) TypeID {
	// NOTE(i4k): several things depends on the fields being sorted by name!
	// - object canonicalization
	// - several places search fields by name using ad-hoc binary search implementations
	// that just assume fields are sorted **by name**, so don't change this lightly!
	sort.Slice(fields, func(i, j int) bool {
		return a.StringValue(fields[i].Name) < a.StringValue(fields[j].Name)
	})

	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(Object))
	a.scratch = put32(a.scratch, int32(len(fields)))
	for _, f := range fields {
		a.scratch = putstr(a.scratch, a.StringValue(f.Name))
		a.scratch = append(a.scratch, byte(f.Flags))
		a.scratch = putu64(a.scratch, a.Fingerprint(f.Value))
	}
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		if n.kind != Object {
			return false
		}
		return equalFields(a.objectFields(id), fields)
	}); id != 0 {
		return id
	}

	r := range32{off: int32(len(a.fields)), len: int32(len(fields))}
	a.fields = append(a.fields, fields...)
	i := int32(len(a.objects))
	a.objects = append(a.objects, r)
	return a.appendNode(Node{kind: Object, data: i}, fp, a.hashHead[fp])
}

func (a *Arena) internSum(members [2]TypeID) TypeID {
	// NOTE(i4k): aggressively intern sum types.
	// Below are the interning rules:
	//   1.  A | A			=> A
	//   2.  A | never		=> A
	//   3.  A | any		=> any
	//   4.  A | B			=> B	iff A <= B
	//   5.  A | B			=> A	iff B <= A
	//
	// special cases:
	//   6.  true | false		=> (bool)
	//   7.  (A, C1) | (A, C2)	=> (A, C1 || C2)
	//
	// TODO(i4k): implement 7.

	flat := make([]TypeID, 0, 8)

	var add func(TypeID)
	// flat the binary tree and handle cases 2 and 3.
	add = func(id TypeID) {
		switch id {
		case 0, a.neverID:
			return
		case a.anyID:
			flat = flat[:0]
			flat = append(flat, a.anyID)
			return
		}
		n := a.Node(id)
		if n.kind == Sum {
			for _, member := range a.sum(id) {
				add(member)
				if len(flat) == 1 && flat[0] == a.anyID {
					return
				}
			}
			return
		}
		flat = append(flat, id)
	}

	add(members[0])
	add(members[1])

	switch len(flat) {
	case 0:
		return a.neverID
	case 1:
		return flat[0]
	}

	var hasTrue, hasFalse bool
	for _, id := range flat {
		n := a.Node(id)
		if n.kind != BoolLit {
			continue
		}
		if n.data != 0 {
			hasTrue = true
		} else {
			hasFalse = true
		}
	}

	// handle case 6.
	if hasTrue && hasFalse {
		out := flat[:0] // reuse flat backing array.
		for _, id := range flat {
			if a.Node(id).kind != BoolLit {
				out = append(out, id)
			}
		}
		flat = append(out, a.boolID)
	}

	sort.Slice(flat, func(i, j int) bool {
		return flat[i] < flat[j]
	})

	out := flat[:0]
	for _, id := range flat {
		if len(out) == 0 || out[len(out)-1] != id {
			out = append(out, id)
		}
	}

	flat = out

	// handle case 4 and 5
	keep := flat[:0]
	for i, id := range flat {
		redundant := false
		for j, other := range flat {
			if i == j {
				continue
			}
			if a.Subtype(id, other) {
				redundant = true
				break
			}
		}
		if !redundant {
			keep = append(keep, id)
		}
	}

	flat = keep

	// handle cases 1 and 2 (after redundant removal)
	switch len(flat) {
	case 0:
		return a.neverID
	case 1:
		return flat[0]
	}

	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(Sum))
	a.scratch = put32(a.scratch, int32(len(flat)))
	for _, id := range flat {
		a.scratch = putu64(a.scratch, a.Fingerprint(id))
	}

	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		return a.nodes[id].kind == Sum && equalTypeIDs(a.sum(id), flat)
	}); id != 0 {
		return id
	}

	r := range32{off: int32(len(a.refs)), len: int32(len(flat))}
	a.refs = append(a.refs, flat...)
	i := int32(len(a.sums))
	a.sums = append(a.sums, r)
	return a.appendNode(Node{kind: Sum, data: i}, fp, a.hashHead[fp])
}

func (a *Arena) internRefined(base TypeID, c ConstraintID) TypeID {
	if c == 0 {
		return base
	}
	a.scratch = a.scratch[:0]
	a.scratch = append(a.scratch, encodingVersion, byte(Refined))
	a.scratch = putu64(a.scratch, a.Fingerprint(base))
	a.scratch = putu64(a.scratch, a.constraintFingerprint(c))
	fp := a.hash(a.scratch)
	if id := a.find(fp, func(id TypeID) bool {
		n := a.nodes[id]
		if n.kind != Refined {
			return false
		}
		r := a.refinements[n.data]
		return r.base == base && r.constraint == c
	}); id != 0 {
		return id
	}
	i := int32(len(a.refinements))
	a.refinements = append(a.refinements, refinement{base: base, constraint: c})
	return a.appendNode(Node{kind: Refined, data: i}, fp, a.hashHead[fp])
}

func (a *Arena) tuple(id TypeID) []TypeID {
	n := a.Node(id)
	if n.kind != Tuple || n.data < 0 || int(n.data) >= len(a.tuples) {
		return nil
	}
	r := a.tuples[n.data]
	return a.refs[r.off : r.off+r.len]
}

func (a *Arena) objectFields(id TypeID) []Field {
	n := a.Node(id)
	if n.kind != Object || n.data < 0 || int(n.data) >= len(a.objects) {
		return nil
	}
	r := a.objects[n.data]
	return a.fields[r.off : r.off+r.len]
}

func (a *Arena) sum(id TypeID) []TypeID {
	n := a.Node(id)
	if n.kind != Sum || int(n.data) >= len(a.sums) {
		return nil
	}
	r := a.sums[n.data]
	return a.refs[r.off : r.off+r.len]
}

func (a *Arena) validTypeID(id TypeID) bool { return id > 0 && int(id) < len(a.nodes) }

func equalTypeIDs(a, b []TypeID) bool {
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

func equalFields(a, b []Field) bool {
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
