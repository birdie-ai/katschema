package katschema

type (
	token  int
	tokval struct {
		kind  token
		start int
		end   int
	}
	tokvals []tokval
)

const (
	eof token = iota
	nul
	tru
	fls
	num
	str
	lbc
	rbc
	lbk
	rbk
	com
)

func eoftok() tokval {
	return tokval{kind: eof}
}
