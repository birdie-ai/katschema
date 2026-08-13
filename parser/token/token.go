// Package token defines lexical tokens and source positions used by the parser.
package token

// Pos is a byte position plus one. Zero means no position.
type Pos uint32

const NoPos Pos = 0

// At converts a zero-based byte offset to a Pos.
func At(off int) Pos {
	if off < 0 {
		return NoPos
	}
	return Pos(off) + 1
}

// Offset returns the zero-based byte offset for p, or -1 for NoPos.
func (p Pos) Offset() int {
	if p == NoPos {
		return -1
	}
	return int(p - 1)
}

// Span is a half-open source range [Start, End).
// A zero Span is unknown, as is common for synthetic AST nodes.
type Span struct {
	Start Pos
	End   Pos
}

func NewSpan(start, end int) Span {
	return Span{Start: At(start), End: At(end)}
}

func (s Span) Valid() bool {
	return s.Start != NoPos && s.End != NoPos && s.Start <= s.End
}

// Kind of the lexical token.
type Kind uint8

const (
	Illegal Kind = iota
	EOF

	Ident
	Number
	String

	Null
	True
	False
	In

	LParen // (
	RParen // )
	LBrack // [
	RBrack // ]
	LBrace // {
	RBrace // }
	Comma  // ,
	Colon  // :
	Dot    // .

	Assign // =
	Eq     // ==
	Ne     // !=
	Lt     // <
	Le     // <=
	Gt     // >
	Ge     // >=
	Match  // ~

	And // &&
	Or  // ||
	Not // !

	Add // +
	Sub // -
	Mul // *
	Div // /
	Mod // %
)

func (k Kind) String() string {
	switch k {
	case EOF:
		return "EOF"
	case Ident:
		return "identifier"
	case Number:
		return "number"
	case String:
		return "string"
	case Null:
		return "null"
	case True:
		return "true"
	case False:
		return "false"
	case In:
		return "in"
	case LParen:
		return "("
	case RParen:
		return ")"
	case LBrack:
		return "["
	case RBrack:
		return "]"
	case LBrace:
		return "{"
	case RBrace:
		return "}"
	case Comma:
		return ","
	case Colon:
		return ":"
	case Dot:
		return "."
	case Assign:
		return "="
	case Eq:
		return "=="
	case Ne:
		return "!="
	case Lt:
		return "<"
	case Le:
		return "<="
	case Gt:
		return ">"
	case Ge:
		return ">="
	case Match:
		return "~"
	case And:
		return "&&"
	case Or:
		return "||"
	case Not:
		return "!"
	case Add:
		return "+"
	case Sub:
		return "-"
	case Mul:
		return "*"
	case Div:
		return "/"
	case Mod:
		return "%"
	}
	return "ILLEGAL"
}

func (k Kind) IsUnary() bool {
	return k == Not || k == Add || k == Sub
}

func (k Kind) IsBinary() bool {
	switch k {
	case Eq, Ne, Lt, Le, Gt, Ge, In, Match,
		And, Or, Add, Sub, Mul, Div, Mod:
		return true
	}
	return false
}
