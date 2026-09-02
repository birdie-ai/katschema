# Katschema

Katschema is a minimalist (yet powerful) schema language (a superset of JSON) for defining the
structure and constraints of data using its shape.

Instead of describing schemas through verbose declarations, Katschema mirrors the
structure of the data itself. This makes schemas easy to read, write, and reason
about while still being expressive and fast to validate data.

It supports the features below:

* Primitive types.
* Structural types (product types).
* Sum types.
* Schema constraints.
* Options.
* Builtin functions.

## Getting Started

For example, if you have the data below:

```json
{
  "id": 137,
  "name": "Richard Feynman",
  "interests": ["physics", "electronics", "lockpicking", "painting"]
}
```

you can define its schema with:

```json
{
  "id": (int),
  "name": (string),
  "interests": [ (string) ]
}
```

The config above tells that `id` has type *int*, `name` has type *string* and `interests` has type
*array of string*.

Now compare the above with JSON-Schema:
```
{
  "type": "object",
  "properties": {
    "id": { "type": "integer" },
    "name": { "type": "string" },
    "interests": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "required": ["id", "name", "interests"]
}
```

You like the idea? Thanks @katcipis for that insight!

As Katschema is a superset of JSON, then all JSON values are valid Katschema. It also means that
you can also carry configuration together with schema definition.
Have a look in this arbitrary config:

```
{
	"types": {
		"user": {
			"id": (string),
			"name": (string, len(x) > 6),
			"birthday": (date, optional)
		}
	},
	"endpoint": "GET /users",
    "response": {
		"schema": [ (.types.user) ] 
	}
}
```

The `types` object is not special, it's just a field. The parenthesis wrap a powerful mini lang
that allows for referencing builtin primitive types or other parts of the config (as for example the
`( .types.user )`.

You can avoid repeating yourself in the config with:
```json
{
	"default": {
		"primary_color": "#ccc",
		"secondary_color": "#ddd"
	},
	"websites": {
		"main": {
			"primary_color": (.default.primary_color ),
			"secondary_color": (.default.secondary_color)
		},
		"landing-page": {
			"primary_color": "#fff",
			"secondary_color": (.default.secondary_color)
		}
	}
}
```

## Concepts

Katschema is easy because it uses the data shape for the schema definition, instead of forcing the
user to declare the types using a [fixed data structure](https://json-schema.org/learn/getting-started-step-by-step#add-the-properties-object)
containing `type`, `properties`, `name`, `description`, etc. If you are used to JSON-Schema you
should know what I mean because in the end, for complex types, the actual config is huge and does
not resemble the actual data.

Using the data shape means that every valid JSON is a valid Katschema definition.
JSON literals represent schemas that match exactly themselves and then the schema language inside
`(...)` is used when a broader set of valid values needs to be described.

So for example if you have a `/health` endpoint that can only ever return one of the two objects
below:

```
{
	"name": "my-service",
	"status": "ok"
}
```

or

```
{
	"name": "my-service",
	"status": "failed"
}
```

Then below is the katschema that fully describe it:

```
{
	"name": "my-service",
	"status": (string, x IN ["ok", "failed"])
}
```

The `x in ["ok", "failed"]` is a value constraint.

Note that `"my-service"` is a literal value and this is a syntax sugar for:

```
(string, x == "my-service")
```
it means the `name` field will **always** be a string with value `"my-service"`.

## Schema lang

The `(` and `)` wraps the schema language. It has the format below:

```
(<type reference>, <clause 1>, <clause 2>, ...)
```

The *type reference* can be:
- primitive type name
- custom type name
- traversal to a value in the same object.

The clauses can be:
- contraint expressions
- key-value config/metadata

Example:

```json
{
	"entity": "user",
	"schema": {
		"id": (uuid, pk=true),
		"name": (string),
		"address": ({
			"street": (string),
			"state": (string),
			"zipcode": (string, match("[a-zA-Z]+-[0-9]+"))
		}, optional)
	}
}
```

## Type ordering

A useful way to think about Katschema is that every value or schema describes a set of valid values.

For example:

```
10
```

only accepts the value `10`, while:

```
(int, x > 0)
```

accepts all positive integers and:

```
(int)
```

accepts all integers.

Because everything accepted by the first schema is also accepted by the next one, we can order
them from more specific to more general:

```
10 <= (int, x > 0), <= (int) <= (any)
```

`any` is the most general type and accepts every value.

The same applies to structured values. For example:

```
{"a": 1} <= {"a": (int)}
```

and

```
[1, 2, 3] <= [(int)]
```

Constraints move a type down in this ordering because they reduce the set of values it accepts:

```
(int, x == 10) <= (int, x > 0) <= (int)
```

**Not every pair of schemas are comparable**. For example:

```
(int, x > 0)
```
and
```
(int, x < 0)
```

are both more specific than `(int)` but neither is more specific than the other.

This kind of ordering is a **partial order** (or **poset**).

### Common types

Given two schemas, Katschema can reason about their common upper and lower bounds.

The **least upper bound**, or `lub`, is the _smallest type_ that can represet values from either
schema.

For example:

```
lub(10, 20)
```

can be represented exactly as:
```
(int, x in [10, 20])
```

and

```
lub((int, x < 0), (int, x >= 0))
```
is just `(int)`, as the smallest type that can represent values both schemas is the whole `integer`
set of values.

For unrelated alternatives, a `sum` type can preserve the exact result:

```
lub((int, x > 0), (string))
```
becomes:
```
(int, x > 0) | (string)
```

Notice that `(any)` would also accept both sides but it _is not_ the _least upper bound_ because
it accepts much more than it's needed.

Similarly:

```
lub((int, x > 0), (int, x < 0))
```

is not necessarily `(int)` because `(int)` also accepts `0`. The exact common type is the sum:
```
(int, x > 0) | (int, x < 0)
```

The other important operation is the **greatest lower bound**, or `glb`, and it means the most
general schema that satisfies both sides at the same time. It's like the intersection of values
that satisfies both types.

For example, `(int, x >= 0)` accepts `0, 1, 2, 3, ..., N` and `(int, x <= 100)` accepts
`-N, ..., -3, -2, -1, 0, 1, 2, 3, ..., 100`, then:

```
glb((int, x >= 0), (int, x <= 100))
```
is
```
(int, 0 <= x <= 100)
```

while:
```
glb((int, x > 0), (int, x < 0))
```
accepts no value (returns Katschema builtin `(never)` type).

In summary:

The `lub(a, b)` makes a type that accept all values of both `a` and `b`.
The `glb(a, b)` makes a type that accepts values that would be accepted by both `a` and `b`.

Together these relationships form a lattice of Katschema values/types.

Apart from validation, the ordering can be used to reason about schema compatibility.
Example:

```
(int, x > 0)
```
to
```
(int)
```
widens the schema, so every value accepted before is still accepted afterwards.
This seems obvious but Katschema gives you a mechanism that expands to all types and then
given any type you can easily check if `old <= new`.
So from `(int)` to `(int, x > 0)` narrows the type, making previously valid values invalid, and
similarly you can apply this to objects and arrays, where each pair of types is either
`incomparable` (which means the change is definitely breaking) or `comparable` and then you
definitily know if it's widening or narrowing the existing type.

### constraint expression

Constraints are boolean expressions evaluated against the value being validated.
Within a constraint expression, the identifier `x` refers to the current value.

Constraints may use arithmetic, comparison, function and logical operators.

#### Operators

Predicates:

* `==`
* `!=`
* `<`
* `<=`
* `>`
* `>=`
* `in`
* `~`

Logical operators:

* `&&`
* `||`
* `!`

Arithmetic operators:

* `+`
* `-`
* `*`
* `/`
* `%`

For example:

```ksc
(int, x > 0)
```

```ksc
(int, x+1 >= 18, optional)
```

```ksc
(float, x >= 0 && x <= 1)
```

```ksc
(int, x % 2 == 0)
```

Snippet above is the same as:
```ksc
(float, 0 <= x <= 1)
```

```ksc
(string, len(x) >= 8)
```

Multiple constraints can be added and it means they all must be true:
For example, the constraint below:
```ksc
(string, len(x) >= 8, len(x) <= 64)
```
is semantically the same as:
```ksc
(string,
    len(x) >= 8 &&
    len(x) <= 64
))
```

Enumeration can be done with the `in` keyword:
```ksc
(string, x in ["admin", "user", "guest"])
```

Syntactic pattern matching:
```ksc
(string, x ~ "^[a-z0-9_]+$")
```

#### Builtin functions

Builtin functions provide helpers and common validators.

#### Len

`len(x)` gives the length of string, array and object (the number of fields):

```ksc
(string, len(x) >= 8)
```

```ksc
([ (string) ], len(x) <= 10)
```

#### Pattern matching

```ksc
(string, x ~ "^[a-z0-9_]+$")
```

is the same as:
```ksc
(string, match(x, "^[a-z0-9_]+$"))
```

#### String predicates

```ksc
(string, startsWith(x, "usr_"))
```

```ksc
(string, endsWith(x, ".png"))
```

```ksc
(string, contains(x, "@"))
```

#### Common formats

Format functions validate and format/normalize/canonicalize the input:

An email can be validated and normalized using `email(x)`:

```ksc
(string, email(x))
```

```ksc
(string, uuid(x))
```

```ksc
(string, url(x))
```

```ksc
(string, ipv4(x))
```

## Examples

```json
{
	"id": (string, uuid(x)),
	"name": (string),
	"email": (string, email(x)),
	"password": (string,
		len(x) >= 8,
		x ~ "[A-Z]", 		// at least 1 uppercase
		x ~ "[a-z]", 		// at least 1 lowercase
		x ~ "[0-9]", 		// at least 1 number
		x ~ "[^A-Za-z0-9]", // at least 1 symbol
	),
	"role": (string, x in ["admin", "user", "guest"]),
	"age": (int, x >= 18, optional=true)
}
```

* `id` is a required UUID string.
* `name` is a required string.
* `email` is a required email address.
* `password` is a required string with at least eight characters and matching multiple patterns.
* `role` must be one of `"admin"`, `"user"` or `"guest"`.
* `age` is an optional integer greater than or equal to `18`.

## Primitive types

The following primitive types are builtin:

* `string`: UTF-8 string.
* `real`: mathematical real number with exact decimal literals and dense bounds.
* `float`: 64-bit floating point.
* `bool`: `true` or `false`.
* `int`: mathematical integer.
* `uint`: alias for `uint64`.
* `int8`, `int16`, `int32`, `int64`: signed integers.
* `uint8`, `uint16`, `uint32`, `uint64`: unsigned integers.

## Collection types

* list
* object
* tuple

### List

In the most basic form, a list is just like a JSON list.
Example:

```json
["this", "is", "a", "list"]
```
The list above is _literal list_.

But if it has the form:
```
[ (<type>) ]
```
Then it's a list container for the provided type. See examples below:

List of strings:
```
[(string)]
```

List of integers:
```
[(int)]
```
etc

### Object

In the most basic form, an object is just like a JSON object.
Example:

```json
{
	"name": "katschema",
	"categories": ["cat", "memes"]
}
```

But in Katschema, each value can also be a schema type. Example:

```json
{
	"name": (string),
	"categories": [(string)]
}
```

### Tuple

A tuple is very similar to a list but it's a **finite** ordered set of values, each value
having its own type.

Example:

Coordinates:
```json
#{29.97416777, 31.1339477975}
```

Coordinate schema:
```json
#{(float), (float)}
```

## Custom types

Schemas may reference other types defined elsewhere:

```
// order.ksc
{
  "id": (string),
  "user": (user),
  "price": (number),
  "products": [ (product) ]
}
```

Note that `user` and `product` are not defined anywhere. It's responsability of the system using
Katschema to ensure these types are loaded.

## Default clauses

Every field has the following default clauses:

* `optional=false`

## Key-value clauses

If a key is provided without a value, it is assumed to be `true`.

For example,

```ksc
(int, optional)
```

is equivalent to

```ksc
(int, optional=true)
```

# Formal specification

Check the [Ohm grammar](./grammar.ohm).
