// Package compiled contains Katschema's canonical semantic representation.
//
// An Arena is an append-only storage of values, most of them interned and int32 ids are
// referenced everywhere, making the trees depending on AST nodes (almost) free of GC pointer scans.
// Types that are semantically equal are interned with same id.
// Types with constraints are refined.
// Refinements with same constraints are interned with same id.
package compiled
