// Package token defines lexical tokens and source positions used by the parser.
package token

// Pos is a zero-based byte offset. Negative positions are invalid.
type Pos int

const NoPos Pos = -1

// At converts a zero-based byte offset to a Pos.
func At(off int) Pos { return Pos(off) }

// Offset returns the zero-based byte offset, preserving negative positions.
func (p Pos) Offset() int { return int(p) }

// Span is a half-open source range [Start, End).
// Its zero value is the valid empty range [0, 0).
type Span struct {
	Start Pos
	End   Pos
}

func NewSpan(start, end int) Span {
	return Span{Start: Pos(start), End: Pos(end)}
}

// UnknownSpan returns the canonical span for synthetic or missing nodes.
func UnknownSpan() Span {
	return Span{Start: NoPos, End: NoPos}
}

func (s Span) Valid() bool {
	return s.Start >= 0 && s.Start <= s.End
}

// Kind of the lexical token.
type Kind uint8

const (
	Illegal Kind = iota
	EOF

	Ident
	Int
	Decimal
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

	Pipe // |

	TupleOpen // #{
	Comment
)

func (k Kind) String() string {
	switch k {
	case EOF:
		return "EOF"
	case Ident:
		return "identifier"
	case Int:
		return "integer"
	case Decimal:
		return "decimal"
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
	case Pipe:
		return "|"
	case TupleOpen:
		return "#{"
	case Comment:
		return "comment"
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
