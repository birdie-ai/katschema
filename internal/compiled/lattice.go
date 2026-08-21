package compiled

// LUB returns the least upper bound of x and y.
//
// The _least upper bound_ is the type that accepts all values from both x and y.
// Also know as Join operation: https://en.wikipedia.org/wiki/Join_and_meet
func (a *Arena) LUB(x, y TypeID) TypeID {
	a.init()

	if !a.validTypeID(x) || !a.validTypeID(y) {
		return 0
	}

	// NOTE(i4k): because we aggressively optimize the interning of SUM types,
	// the LUB(x, y) == SUM(x, y) and it's reduced to either X or Y depending
	// on if any of them is the supremum of the other.
	return a.internSum([2]TypeID{x, y})
}
