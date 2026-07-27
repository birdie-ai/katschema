package katschema

import (
	"bytes"
	"fmt"

	"github.com/birdie-ai/golibs/obj"
)

type (
	Expr struct {
		Type Type
	}

	Ref struct {
		T Type
		O *Object
		L *List
		M *Map
		B *Bool
		I *Int
		U *Uint
		F *Float
		S *String
	}

	Object struct {
		Alias   string
		Fields  map[string]Type
		Options obj.O
	}

	List struct {
		Alias   string
		Items   Type
		Options obj.O
	}

	Bool struct {
		Alias   string
		Options obj.O
	}

	Int struct {
		Alias   string
		Options obj.O
	}

	Uint struct {
		Alias   string
		Options obj.O
	}

	Float struct {
		Alias   string
		Options obj.O
	}

	String struct {
		Alias   string
		Options obj.O
	}

	AnalyzedText String

	Date struct {
		Alias   string
		Format  string
		Options obj.O
	}

	Datetime struct {
		Alias   string
		Options obj.O
	}

	Map struct {
		Alias   string
		Options obj.O
		Key     Type
		Value   Type
	}

	Sum struct {
		Alias    string
		Options  obj.O
		Variants Types
	}

	Product struct {
		Alias    string
		Options  obj.O
		Operands Types
	}

	Collection interface {
		Type
		Get(path Path) (Type, error)
	}

	Types []Type
)

var (
	_ = Type(Bool{})
	_ = Type(Int{})
	_ = Type(Float{})
	_ = Type(String{})
	_ = Type(AnalyzedText{})
	_ = Type(Date{})
	_ = Type(Datetime{})
	_ = Type(Object{})
	_ = Type(List{})
	_ = Type(Map{})
	_ = Type(Sum{})
	_ = Type(Product{})
	_ = Type(Enum{})

	_ = Collection(Object{})
	_ = Collection(List{})
)

func (t Bool) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "bool"
}

func (t Bool) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Int) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "int"
}

func (t Int) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Float) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "float"
}

func (t Float) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t String) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "string"
}

func (t String) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t AnalyzedText) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "analyzed_text"
}

func (t AnalyzedText) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Date) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "date"
}

func (t Date) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Datetime) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "datetime"
}

func (t Datetime) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Object) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "object"
}

func (t Object) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Object) Get(path Path) (Type, error) {
	ftype, ok := t.Fields[path[0]]
	if !ok {
		return nil, ErrPathNotFound
	}
	if len(path) == 1 {
		return ftype, nil
	}
	fcol, ok := ftype.(Collection)
	if !ok {
		return nil, fmt.Errorf(
			`%w: path %q address inside non-collection type %s`,
			ErrPathNotFound, path.String(), ftype.Name(),
		)
	}
	return fcol.Get(path[1:])
}

func (t List) Get(path Path) (Type, error) {
	col, ok := t.Items.(Collection)
	if !ok {
		return nil, fmt.Errorf(
			`%w: path %q address inside non-collection type %s`,
			ErrPathNotFound, path.String(), t.Items.Name(),
		)
	}
	return col.Get(path)
}

func (t List) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return "list"
}

func (t List) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Sum) Name() string {
	var name string
	if t.Alias != "" {
		name = t.Alias
	} else {
		name = "sum"
	}
	vlistbuf := make([]byte, 0, len(t.Variants)*20)
	buf := bytes.NewBuffer(vlistbuf)
	for i, v := range t.Variants {
		if i > 0 {
			_, _ = buf.WriteString(" | ")
		}
		_, _ = buf.WriteString(v.Name())
	}
	return fmt.Sprintf("%s(%s)", name, buf.String())
}

func (t Sum) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Product) Name() string {
	var name string
	if t.Alias != "" {
		name = t.Alias
	} else {
		name = "product"
	}
	vlistbuf := make([]byte, 0, len(t.Operands)*20)
	buf := bytes.NewBuffer(vlistbuf)
	for i, v := range t.Operands {
		if i > 0 {
			_, _ = buf.WriteString(" | ")
		}
		_, _ = buf.WriteString(v.Name())
	}
	return fmt.Sprintf("%s(%s)", name, buf.String())
}

func (t Product) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Map) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return fmt.Sprintf("map(%s, %s)", t.Key.Name(), t.Value.Name())
}

func (t Map) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}

func (t Enum) Name() string {
	if t.Alias != "" {
		return t.Alias
	}
	return fmt.Sprintf("enum(%s, %v)", t.Type.Name(), fmt.Sprint(t.Values))
}

func (t Enum) Option(path string) (any, bool) {
	val, err := obj.Get[any](t.Options, path)
	return val, err == nil
}
