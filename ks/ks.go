// Package ks provides a small Go DSL for constructing Katschema syntax trees.
package ks

import (
	"fmt"
	"strconv"

	"github.com/birdie-ai/katschema/parser/ast"
	"github.com/birdie-ai/katschema/parser/token"
)

// NOTE(i4k): most types use tagged union for the reasons below:
// 1. they are short-lived and naturally tagged (its kind), so interface based type switch is wasteful.
// 2. as they are short-lived, we can (mostly) avoid pointer fields and then reduce GC scans (they will be all freed together when root is collected).
// 3. make traversal consistent.

type (
	Value struct {
		kind   valueKind
		text   string
		b      bool
		elems  []Value
		fields []Member
		schema *schemaSpec
	}

	Member struct {
		name  string
		value Value
	}

	Clause struct {
		kind     clauseKind
		name     string
		value    Expr
		hasValue bool
	}

	Expr struct {
		kind exprKind
		name string
		path []string
		args []Expr
		v    *Value
		op   token.Kind
		x    *Expr
		y    *Expr
	}
)

// NOTE(i4k); tokens are re-exported in this package so the user don't need to import the
// [parser/token] package which is low-level and prone to change. Now they map one-to-one
// but we can evolve then separately in the future, if needed.

type Op = token.Kind

const (
	Eq    = token.Eq
	Ne    = token.Ne
	Lt    = token.Lt
	Le    = token.Le
	Gt    = token.Gt
	Ge    = token.Ge
	In    = token.In
	Match = token.Match
	And   = token.And
	Or    = token.Or
	Not   = token.Not
	Add   = token.Add
	Sub   = token.Sub
	Mul   = token.Mul
	Div   = token.Div
	Mod   = token.Mod
)

// Funcs below are common types.

func Any() Value    { return Type("any") }
func Never() Value  { return Type("never") }
func Bool() Value   { return Type("bool") }
func Int() Value    { return Type("int") }
func Float() Value  { return Type("float") }
func String() Value { return Type("string") }

func X() Expr { return Ident("x") }

func Type(name string) Value {
	return Value{
		kind: valueSchema,
		schema: &schemaSpec{
			ref: typeRef{kind: refName, name: name},
		},
	}
}

// Ref returns a schema referring to a path such as (.types.user).
func Ref(parts ...string) Value {
	return Value{
		kind: valueSchema,
		schema: &schemaSpec{
			ref: typeRef{kind: refPath, path: clone(parts)},
		},
	}
}

// Literals have Lit* prefix because I decided to favor the Null(), Bool(), Int(), etc to the
// type constructors.

func LitNull() Value {
	return Value{kind: valueNull}
}

func LitBool(v bool) Value {
	return Value{kind: valueBool, b: v}
}

// LitInt is a literal int.
func LitInt(i int64) Value {
	return Value{kind: valueInt, text: strconv.FormatInt(i, 10)}
}

// LitFloat is a literal float.
func LitFloat(f float64) Value {
	return Value{kind: valueFloat, text: strconv.FormatFloat(f, 'f', -1, 64)}
}

func LitString(v string) Value {
	return Value{kind: valueString, text: v}
}

func List(elems ...Value) Value {
	return Value{kind: valueArray, elems: clone(elems)}
}

func Field(name string, value Value) Member {
	return Member{name: name, value: value}
}

func Object(fields ...Member) Value {
	return Value{kind: valueObject, fields: clone(fields)}
}

// With adds clauses to a value. If the value is already a schema, then it append the
// provided clauses to the schema otherwise it wraps the value in a literal schema.
func With(v Value, clauses ...Clause) Value {
	if v.kind == valueSchema && v.schema != nil {
		s := *v.schema
		s.clauses = append(clone(s.clauses), clauses...)
		v.schema = &s
		return v
	}

	vv := v
	return Value{
		kind: valueSchema,
		schema: &schemaSpec{
			ref:     typeRef{kind: refValue, value: &vv},
			clauses: clone(clauses),
		},
	}
}

// Optional makes the value optional.
func Optional(v Value) Value {
	return With(v, Flag("optional"))
}

func Where(v Value, e Expr) Value {
	return With(v, Check(e))
}

func Check(e Expr) Clause {
	return Clause{kind: clauseConstraint, value: e, hasValue: true}
}

func Flag(name string) Clause {
	return Clause{kind: clauseAttr, name: name}
}

func Attr(name string, value Expr) Clause {
	return Clause{kind: clauseAttr, name: name, value: value, hasValue: true}
}

func Ident(name string) Expr {
	return Expr{kind: exprIdent, name: name}
}

func ExprPath(parts ...string) Expr {
	return Expr{kind: exprPath, path: clone(parts)}
}

func ValueExpr(v Value) Expr {
	vv := v
	return Expr{kind: exprValue, v: &vv}
}

func Str(v string) Expr {
	return ValueExpr(LitString(v))
}

func Boolean(v bool) Expr {
	return ValueExpr(LitBool(v))
}

func NullExpr() Expr {
	return ValueExpr(LitNull())
}

func IntExpr(v Value) Expr {
	return ValueExpr(v)
}

func ListExpr(v ...Value) Expr {
	return ValueExpr(List(v...))
}

func Call(name string, args ...Expr) Expr {
	return Expr{kind: exprCall, name: name, args: clone(args)}
}

func Unary(op Op, x Expr) Expr {
	xx := x
	return Expr{kind: exprUnary, op: op, x: &xx}
}

// Binary is syntactic. It does not infer or insert grouping.
func Binary(left Expr, op Op, right Expr) Expr {
	l, r := left, right
	return Expr{kind: exprBinary, op: op, x: &l, y: &r}
}

func Group(x Expr) Expr {
	xx := x
	return Expr{kind: exprGroup, x: &xx}
}

// Build creates a synthetic AST. All source spans are zero.
func Build(v Value) (*ast.Tree, ast.NodeID, error) {
	t := ast.New()
	n, err := emitValue(t, v)
	if err != nil {
		return nil, 0, err
	}
	return t, n, nil
}

type valueKind uint8

const (
	valueInvalid valueKind = iota
	valueNull
	valueBool
	valueInt
	valueFloat
	valueString
	valueArray
	valueObject
	valueSchema
)

type refKind uint8

const (
	refInvalid refKind = iota
	refName
	refPath
	refValue
)

type schemaSpec struct {
	ref     typeRef
	clauses []Clause
}

type typeRef struct {
	kind  refKind
	name  string
	path  []string
	value *Value
}

type clauseKind uint8

const (
	clauseInvalid clauseKind = iota
	clauseConstraint
	clauseAttr
)

type exprKind uint8

const (
	exprInvalid exprKind = iota
	exprValue
	exprIdent
	exprPath
	exprCall
	exprUnary
	exprBinary
	exprGroup
)

func emitValue(t *ast.Tree, v Value) (ast.NodeID, error) {
	var (
		z   token.Span
		err error
	)

	switch v.kind {
	case valueNull:
		return t.AddNull(z), nil
	case valueBool:
		return t.AddBool(v.b, z), nil
	case valueInt:
		return t.AddInt(v.text, z), nil
	case valueFloat:
		return t.AddFloat(v.text, z), nil
	case valueString:
		return t.AddString(v.text, z), nil
	case valueArray:
		elems := make([]ast.NodeID, len(v.elems))
		for i := range v.elems {
			elems[i], err = emitValue(t, v.elems[i])
			if err != nil {
				return 0, err
			}
		}
		return t.AddList(elems, z), nil
	case valueObject:
		fields := make([]ast.Field, len(v.fields))
		for i := range v.fields {
			id, err := emitValue(t, v.fields[i].value)
			if err != nil {
				return 0, err
			}
			fields[i] = t.NewField(v.fields[i].name, id, z, z)
		}
		return t.AddObject(fields, z), nil
	case valueSchema:
		return emitSchema(t, *v.schema)
	default:
		panic(fmt.Errorf("ks: invalid valuekind %v", v.kind))
	}
}

func emitSchema(t *ast.Tree, s schemaSpec) (ast.NodeID, error) {
	var z token.Span

	typ, err := emitRef(t, s.ref)
	if err != nil {
		return 0, err
	}

	clauses := make([]ast.NodeID, len(s.clauses))
	for i := range s.clauses {
		id, err := emitClause(t, s.clauses[i])
		if err != nil {
			return 0, err
		}
		clauses[i] = id
	}
	return t.AddSchema(typ, clauses, z), nil
}

func emitRef(t *ast.Tree, r typeRef) (ast.NodeID, error) {
	var z token.Span

	switch r.kind {
	case refName:
		if r.name == "" {
			return 0, fmt.Errorf("ks: empty type name")
		}
		return t.AddName(r.name, z), nil
	case refPath:
		if len(r.path) == 0 {
			return 0, fmt.Errorf("ks: empty type path")
		}
		parts := make([]ast.PathPart, len(r.path))
		for i := range r.path {
			parts[i] = t.NewPathPart(r.path[i], z)
		}
		return t.AddPath(parts, z), nil
	case refValue:
		if r.value == nil {
			return 0, fmt.Errorf("ks: invalid type reference")
		}
		id, err := emitValue(t, *r.value)
		if err != nil {
			return 0, err
		}
		return id, nil
	}

	return 0, fmt.Errorf("ks: invalid type reference")
}

func emitClause(t *ast.Tree, c Clause) (ast.NodeID, error) {
	var z token.Span

	switch c.kind {
	case clauseConstraint:
		e, err := emitExpr(t, c.value)
		if err != nil {
			return 0, err
		}
		return t.AddConstraint(e, z), nil
	case clauseAttr:
		if c.name == "" {
			return 0, fmt.Errorf("ks: empty attribute name")
		}
		if !c.hasValue {
			return t.AddAttr(c.name, 0, false, z, z), nil
		}
		e, err := emitExpr(t, c.value)
		if err != nil {
			return 0, err
		}
		return t.AddAttr(c.name, e, true, z, z), nil
	}

	return 0, fmt.Errorf("ks: invalid clause")
}

func emitExpr(t *ast.Tree, e Expr) (ast.NodeID, error) {
	var z token.Span

	switch e.kind {
	case exprValue:
		id, err := emitValue(t, *e.v)
		if err != nil {
			return 0, err
		}
		if !t.Node(id).Kind().IsExpr() {
			return 0, fmt.Errorf("ks: %s cannot be used as an expression", t.Node(id).Kind())
		}
		return id, nil
	case exprIdent:
		return t.AddIdent(e.name, z), nil
	case exprPath:
		parts := make([]ast.PathPart, len(e.path))
		for i := range e.path {
			parts[i] = t.NewPathPart(e.path[i], z)
		}
		return t.AddPath(parts, z), nil
	case exprCall:
		args := make([]ast.NodeID, len(e.args))
		for i := range e.args {
			id, err := emitExpr(t, e.args[i])
			if err != nil {
				return 0, err
			}
			args[i] = id
		}
		return t.AddCall(e.name, args, z, z), nil
	case exprUnary:
		if !e.op.IsUnary() || e.x == nil {
			return 0, fmt.Errorf("ks: invalid unary expression")
		}
		x, err := emitExpr(t, *e.x)
		if err != nil {
			return 0, err
		}
		return t.AddUnary(e.op, x, z), nil
	case exprBinary:
		if !e.op.IsBinary() || e.x == nil || e.y == nil {
			return 0, fmt.Errorf("ks: invalid binary expression")
		}
		left, err := emitExpr(t, *e.x)
		if err != nil {
			return 0, err
		}
		right, err := emitExpr(t, *e.y)
		if err != nil {
			return 0, err
		}
		return t.AddBinary(left, e.op, right, z), nil
	case exprGroup:
		x, err := emitExpr(t, *e.x)
		if err != nil {
			return 0, err
		}
		return t.AddGroup(z, x), nil
	}
	return 0, fmt.Errorf("ks: invalid expression")
}

func clone[T any](v []T) []T {
	if v == nil {
		return nil
	}
	if len(v) == 0 {
		var z []T
		return z
	}
	return append([]T(nil), v...)
}
