package compile

import (
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"
)

type TSVar struct {
	Type     string
	Nillable bool
	Items    *TSVar
	Key      *TSVar
}

func (ts TSVar) Render() string {
	if ts.Nillable {
		return "(" + ts.renderType() + " | null)"
	}
	return ts.renderType()
}

func (ts TSVar) renderType() string {
	if ts.Type != "" {
		return ts.Type
	}
	if ts.Key != nil {
		return "{[key: " + ts.Key.Render() + "]: " + ts.Items.Render() + "}"
	}
	return ts.Items.Render() + "[]"
}

type Param struct {
	Name     string
	Source   *types.Var
	Optional bool
	TS       TSVar
}

type Type struct {
	Source types.Type
	TS     TSVar
}

type Method struct {
	Name        string
	Description string
	Source      *types.Func
	Args        []Param
	Result      *Type // may be nil
}

func (m *Method) ArgNames() []string {
	var ans = make([]string, 0, len(m.Args))
	for _, a := range m.Args {
		ans = append(ans, a.Name)
	}
	return ans
}

type API struct {
	Name        string
	Description string
	Methods     []*Method
}

func New() *TypeLookup {
	return &TypeLookup{
		customTypes:    map[string]TSVar{},
		registeredType: map[string]string{},
		typesNames:     map[string]int{},
		typeAliases:    map[string]Type{},
		typeObjects:    map[string][]Param{},
		comments: func(pos token.Pos) string {
			return ""
		},
	}
}

type TypeLookup struct {
	customTypes    map[string]TSVar  // user-defined mapping
	registeredType map[string]string // fqdn -> alias
	typesNames     map[string]int    // name -> frequency (to avoid collision)

	typeAliases map[string]Type    // type Foo = Bar
	typeObjects map[string][]Param // type Foo struct
	comments    func(pos token.Pos) string
}

func (tl *TypeLookup) CommentLookup(handler func(pos token.Pos) string) {
	tl.comments = handler
}

func (tl *TypeLookup) ScanAPI(obj *types.Named) API {
	var api = API{
		Name:        tl.allocateTypeName(obj.Obj().Name()),
		Description: tl.comments(obj.Obj().Pos()),
	}
	for i := 0; i < obj.NumMethods(); i++ {
		m := obj.Method(i)
		fn, ok := tl.scanMethod(m)
		if !ok {
			continue
		}
		api.Methods = append(api.Methods, fn)
	}
	return api
}

func (tl *TypeLookup) Aliases() map[string]Type {
	return tl.typeAliases
}

func (tl *TypeLookup) Objects() map[string][]Param {
	return tl.typeObjects
}

func (tl *TypeLookup) RegisterType(obj *types.Named) string {
	alias, ok := tl.registeredType[obj.String()]
	if ok {
		return alias
	}
	typeName := tl.allocateTypeName(obj.Obj().Name())
	tl.registeredType[obj.String()] = typeName

	tl.defineTypes(typeName, obj.Obj().Type().Underlying())
	return typeName
}

func (tl *TypeLookup) CastToTypesScript(src types.Type) TSVar {
	switch t := src.(type) {
	case *types.Basic:
		info := t.Info()
		switch {
		case info&types.IsBoolean != 0:
			return TSVar{Type: "boolean"}
		case info&(types.IsNumeric|types.IsUnsigned|types.IsUntyped) != 0:
			return TSVar{Type: "number"}
		case info&(types.IsString) != 0:
			return TSVar{Type: "string"}
		default:
			return TSVar{Type: "any"}
		}
	case *types.Interface:
		return TSVar{Type: "any"}
	case *types.Array:
		if isByteArray(t) {
			return TSVar{Type: "string"}
		}
		items := tl.CastToTypesScript(t.Elem())
		return TSVar{Items: &items}
	case *types.Slice:
		if isByteArray(t) {
			return TSVar{Type: "string"}
		}
		items := tl.CastToTypesScript(t.Elem())
		return TSVar{Items: &items}
	case *types.Pointer:
		r := tl.CastToTypesScript(t.Elem())
		r.Nillable = true
		return r
	case *types.Map:
		key := tl.CastToTypesScript(t.Key())
		value := tl.CastToTypesScript(t.Elem())
		return TSVar{Key: &key, Items: &value}
	case *types.Named:
		// import or declaration
		obj := t.Obj()
		pkg := obj.Pkg()
		switch {
		case pkg.Path() == "time" && obj.Name() == "Time":
			return TSVar{Type: "string"}
		case pkg.Path() == "time" && obj.Name() == "Duration":
			return TSVar{Type: "string"}
		case pkg.Path() == "github.com/shopspring/decimal" && obj.Name() == "Decimal":
			return TSVar{Type: "string"}
		}
		custom, ok := tl.customTypes[obj.Type().String()]
		if ok {
			return custom
		}
		return TSVar{Type: tl.RegisterType(t)}
	case *types.Struct:
		return TSVar{Type: tl.defineAnonType(t)}
	default:
		panic("unsupported type mapping for " + t.String())
	}
}

func (tl *TypeLookup) defineTypes(name string, obj types.Type) {
	if st, ok := obj.(*types.Struct); ok {
		// object definition
		tl.typeObjects[name] = tl.defineStruct(st)
	} else {
		// type alias
		tl.typeAliases[name] = Type{TS: tl.CastToTypesScript(obj), Source: st}
	}
}

func (tl *TypeLookup) allocateTypeName(name string) string {
	alias := name
	if n := tl.typesNames[name]; n > 0 {
		alias += strconv.Itoa(n)
	}
	tl.typesNames[name] += 1
	return alias
}

func (tl *TypeLookup) defineAnonType(obj *types.Struct) string {
	typeName := tl.allocateTypeName("Anon")
	tl.defineTypes(typeName, obj)
	return typeName
}

func (tl *TypeLookup) defineStruct(obj *types.Struct) []Param {
	var ans []Param
	for i := 0; i < obj.NumFields(); i++ {
		field := obj.Field(i)
		if !field.Exported() {
			continue
		}
		tag := reflect.StructTag(obj.Tag(i))
		name, opts, _ := strings.Cut(tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name()
		}
		optional := strings.Contains(opts, "omitempty")
		ans = append(ans, Param{
			Name:     name,
			Source:   field,
			Optional: optional,
			TS:       tl.CastToTypesScript(field.Type()),
		})
	}
	return ans
}

func isByteArray(t interface{ Elem() types.Type }) bool {
	v, ok := t.Elem().(*types.Basic)
	if !ok {
		return false
	}
	return v.Kind() == types.Byte
}

func (tl *TypeLookup) scanMethod(m *types.Func) (*Method, bool) {
	if !m.Exported() {
		return nil, false
	}
	var fn = Method{Source: m, Name: m.Name(), Description: tl.comments(m.Pos())}
	sig := m.Type().Underlying().(*types.Signature)
	resN := sig.Results().Len()
	if resN > 2 {
		return nil, false
	}
	// if out=2, then the last always should be an error
	if resN == 2 {
		if !isError(sig.Results().At(1).Type()) {
			return nil, false
		}
		t := sig.Results().At(0).Type()
		fn.Result = &Type{Source: t, TS: tl.CastToTypesScript(t)}
	}
	// if out=1, then the output should be only if it's not an error
	if resN == 1 && !isError(sig.Results().At(0).Type()) {
		t := sig.Results().At(0).Type()
		fn.Result = &Type{Source: t, TS: tl.CastToTypesScript(t)}
	}

	for i := 0; i < sig.Params().Len(); i++ {
		arg := sig.Params().At(i)
		if isContext(arg.Type()) {
			continue
		}
		fn.Args = append(fn.Args, Param{
			Name:   arg.Name(),
			Source: arg,
			TS:     tl.CastToTypesScript(arg.Type()),
		})
	}

	return &fn, true
}

func isError(tp types.Type) bool {
	nm, ok := tp.(*types.Named)
	if !ok {
		return false
	}
	return nm.Obj().Pkg() == nil && nm.Obj().Name() == "error"
}

func isContext(tp types.Type) bool {
	nm, ok := tp.(*types.Named)
	if !ok {
		return false
	}
	return nm.Obj().Type().String() == "context.Context"
}
