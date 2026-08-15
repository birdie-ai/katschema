# canonical AST

Why we need a canonical (compiled) AST?

Katschema is designed to be used as the schema language for defining entities in a proprietary
database and because of that we should be able to compare entity changes and answer the questions
below:

1. is the new schema semantically different than current applied one?
2. is the new schema going to break writers or readers? or better, is it a breaking changes?
3. is the new schema widening the accepted values? or is it narrowing the accepted values?
	- if it's narrowing, is it incompatible with saved data?

In order to answer these questions, fundamentally, the system needs to canonicalize the schema
definitions so we can compare only the semantic differences of new and old schema.
Just to give a quick example, let's say an entity called `feedback` has schema below:

```json
{
	"id": (string, pk),
	"text": (string, optional),
	"kind": (string, x IN ["support_ticket", "nps", "complaint", "review"]),
	"rating": (int, 0 <= x <= 10)
}
```

and the user submits the new schema below:

```json
{
	"id": (string, pk),
	"rating": (int, -1 < x < 11),
	"kind": (string, x IN ["nps", "support_ticket", "complaint", "review"]),
	"text": (string, optional),
}
```

If you pay attention you notice that they are semantically the same.
The cases above are simple to detect but that's not always the case, sometimes it's very
difficult and in some even impossible. Below is an slightly harder case but solvable:

```json
{
	"rating": (int, x IN [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10]),
	"kind": (string, x == "nps" || x == "support_ticket" || x == "complaint" || x == "review"),
	"text": (string, optional),
	"id": (string, pk)
}
```

For integers, an `IN` clause with values encompassing a range with no gaps, is semantically the
same as a range constraint. Similarly, an array of OR branches, each with equality predicate, is
semantically the same as a `IN` clause.
Katschema is designed to be easy to write by hand, and also easy to read, so user is free to
choose from several different ways of describing their constraints. This freedom has the downside
of requiring an advanced internal representation of the AST.

The example above covers just the easy aspect of the schema changes, the other questions related
to "widening" or "narrowing" of schemas are solved by lattices but a canonical representation is
the basis for having it.
