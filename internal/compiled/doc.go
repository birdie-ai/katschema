// Package compiled contains Katschema's canonical semantic representation.
//
// An Arena stores append-only, interned definitions using local int32 IDs.
// Type retains the owning arena and can compose references from multiple arenas.
// There are two equality concepts:
// - definition equality
// - semantic equivalence
// The definition equality within same arena is an ID equality.
// The semantic equivalance compare across arenas and use Type.SubtypeOf and Type.Equivalent.
// The names and metadata belong to definition identity, not semantic equivalence.
package compiled
