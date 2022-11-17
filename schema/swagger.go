// Package schema defines basic OpenAPI schema for RPC
package schema

import (
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/reddec/rpc"
)

// Schema represents OpenAPI definition of endpoints.
type Schema struct {
	OpenAPI string `json:"openapi" yaml:"openapi"`
	Info    struct {
		Title   string `json:"title" yaml:"title"`
		Version string `json:"version" yaml:"version"`
	} `json:"info" yaml:"info"`
	Paths      map[string]Path `json:"paths,omitempty" yaml:"paths,omitempty"`
	Components struct {
		Schemas map[string]*Type `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	} `json:"components,omitempty" yaml:"components,omitempty"`
}

type Path struct {
	Post Endpoint `json:"post" yaml:"post"`
}

type Endpoint struct {
	Summary     string  `json:"summary,omitempty" yaml:"summary,omitempty"`
	OperationID string  `json:"operationId" yaml:"operationId"`
	RequestBody Payload `json:"requestBody" yaml:"requestBody"`
	Responses   struct {
		OK            Payload  `json:"200" yaml:"200"`
		BadRequest    *Payload `json:"400" yaml:"400"`
		InternalError *Payload `json:"500" yaml:"500"`
	} `json:"responses" yaml:"responses"`
}

type ContentType struct {
	Schema *Type `json:"schema,omitempty" yaml:"schema,omitempty"`
}

type Payload struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Content     struct {
		JSON  *ContentType `json:"application/json,omitempty" yaml:"application/json,omitempty"`
		Plain *ContentType `json:"text/plain,omitempty" yaml:"text/plain,omitempty"`
	} `json:"content,omitempty" yaml:"content,omitempty"`
}

type Type struct {
	Type        string           `json:"type,omitempty" yaml:"type,omitempty"`
	Format      string           `json:"format,omitempty" yaml:"format,omitempty"`
	Ref         string           `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Items       *Type            `json:"items,omitempty" yaml:"items,omitempty"`
	Properties  map[string]*Type `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string         `json:"required,omitempty" yaml:"required,omitempty"`
	Minimum     *int64           `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum     int64            `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	PrefixItems []*Type          `json:"prefixItems,omitempty" yaml:"prefixItems,omitempty"`
	MinItems    int              `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	MaxItems    int              `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty"`
	Name        string           `json:"-" yaml:"-"`
}

// Option configures schema creation.
type Option func(builder *schemaBuilder)

// OpenAPI generates Open-API 3.1 schema based on pre-indexed server object (see [rpc.Index]).
// It's recommend to cache result.
func OpenAPI(index map[string]*rpc.ExposedMethod, options ...Option) *Schema {
	var zero = new(int64)
	sb := schemaBuilder{
		components: make(map[schemaRef]*Type),
		names:      make(map[string]int),
		hooks: map[schemaRef]*Type{
			{pkg: "time", name: "Time"}:                             {Type: "string", Format: "date-time"},
			{pkg: "time", name: "Duration"}:                         {Type: "string", Description: "duration with unit prefix"},
			{pkg: "github.com/shopspring/decimal", name: "Decimal"}: {Type: "string", Description: "precise representation of decimal value"},
		},
		defaults: schemaDefaults{
			// defaults avoids creating same type,
			// reduces memory allocation, and allows customization
			Int:   &Type{Type: "integer"},
			Int64: &Type{Type: "integer", Format: "int64"},
			Int32: &Type{Type: "integer", Format: "int32"},
			Int16: &Type{Type: "integer", Maximum: math.MaxInt16},
			Int8:  &Type{Type: "integer", Maximum: math.MaxInt8},

			UInt:   &Type{Type: "integer", Minimum: zero},
			UInt64: &Type{Type: "integer", Format: "int64", Minimum: zero},
			UInt32: &Type{Type: "integer", Format: "int32", Minimum: zero},
			UInt16: &Type{Type: "integer", Minimum: zero, Maximum: math.MaxUint16},
			UInt8:  &Type{Type: "integer", Minimum: zero, Maximum: math.MaxUint8},

			String:  &Type{Type: "string"},
			Bool:    &Type{Type: "boolean"},
			Float32: &Type{Type: "number", Format: "float"},
			Float64: &Type{Type: "number", Format: "double"},

			Base64: &Type{Type: "string", Format: "byte"},
			Any:    &Type{},
		},
	}

	for _, opt := range options {
		opt(&sb)
	}
	schema := sb.build(index)
	return schema
}

type schemaRef struct {
	pkg  string
	name string
}

func refOf(t reflect.Type) schemaRef {
	return schemaRef{
		pkg:  t.PkgPath(),
		name: t.Name(),
	}
}

type schemaDefaults struct {
	Int   *Type
	Int64 *Type
	Int32 *Type
	Int16 *Type
	Int8  *Type

	UInt   *Type
	UInt64 *Type
	UInt32 *Type
	UInt16 *Type
	UInt8  *Type

	String  *Type
	Bool    *Type
	Float32 *Type
	Float64 *Type

	Base64 *Type
	Any    *Type
}

type schemaBuilder struct {
	title      string
	version    string
	components map[schemaRef]*Type
	names      map[string]int
	hooks      map[schemaRef]*Type
	defaults   schemaDefaults
}

func (sb *schemaBuilder) walk(t reflect.Type) *Type {
	if mock, exists := sb.hooks[refOf(t)]; exists {
		return mock
	}
	switch t.Kind() {
	case reflect.Ptr:
		return sb.walk(t.Elem())
	case reflect.Int:
		return sb.defaults.Int
	case reflect.Int64:
		return sb.defaults.Int64
	case reflect.Int32:
		return sb.defaults.Int32
	case reflect.Int16:
		return sb.defaults.Int16
	case reflect.Int8:
		return sb.defaults.Int8

	case reflect.Uint:
		return sb.defaults.UInt
	case reflect.Uint64:
		return sb.defaults.UInt64
	case reflect.Uint32:
		return sb.defaults.UInt32
	case reflect.Uint16:
		return sb.defaults.UInt16
	case reflect.Uint8:
		return sb.defaults.UInt8
	case reflect.String:
		return sb.defaults.String
	case reflect.Bool:
		return sb.defaults.Bool
	case reflect.Float32:
		return sb.defaults.Float32
	case reflect.Float64:
		return sb.defaults.Float64
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			// base64
			return sb.defaults.Base64
		}
		return &Type{Type: "array", Items: sb.walk(t.Elem())}
	case reflect.Array:
		return &Type{Type: "array", Items: sb.walk(t.Elem()), MinItems: t.Len(), MaxItems: t.Len()}
	case reflect.Struct:
		if t.Name() == "" { //anonymous, we need to embed
			return sb.walkStruct(t)
		}
		info := sb.walkStruct(t)
		return &Type{Ref: "#/components/schemas/" + info.Name}
	default:
		return sb.defaults.Any
	}
}

func (sb *schemaBuilder) walkStruct(t reflect.Type) *Type {
	ref := refOf(t)
	var anonymous = ref.name == ""

	if !anonymous {
		if parsed, ok := sb.components[ref]; ok {
			return parsed
		}
	}

	cardinality := sb.names[t.Name()]
	sb.names[t.Name()] += 1

	n := t.NumField()
	var res = &Type{
		Type:       "object",
		Properties: make(map[string]*Type, n),
	}
	if cardinality == 0 {
		res.Name = t.Name()
	} else {
		res.Name = t.Name() + strconv.Itoa(n)
	}

	if !anonymous {
		// enable self reference
		sb.components[ref] = res
	}

	for i := 0; i < n; i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		value := f.Tag.Get("json")
		if value == "-" {
			continue
		}
		if value == "" {
			value = f.Name
		}
		res.Properties[value] = sb.walk(f.Type)
	}
	return res
}

func (sb *schemaBuilder) walkMethodArgs(method *rpc.ExposedMethod) *Type {
	var res = &Type{
		Type:     "array",
		MinItems: len(method.Args()),
		MaxItems: len(method.Args()),
		Items:    sb.defaults.Any,
	}
	for _, arg := range method.Args() {
		res.PrefixItems = append(res.PrefixItems, sb.walk(arg))
	}
	return res
}

func (sb *schemaBuilder) build(index map[string]*rpc.ExposedMethod) *Schema {
	var schema = Schema{
		OpenAPI: "3.1.0",
		Paths:   map[string]Path{},
	}

	// we are preparing all response types since they all the same for all endpoints.
	var errorType = &ContentType{Schema: sb.defaults.String}

	var badRequest = &Payload{
		Description: "Payload can not be unmarshalled to arguments or number of arguments not enough, returns error message (plain text)",
	}
	badRequest.Content.Plain = errorType

	var internalError = &Payload{
		Description: "Method returned an error or factory returned error, returns error message (plain text)",
	}
	internalError.Content.Plain = errorType

	for method, info := range index {
		var path Path

		path.Post.OperationID = method
		path.Post.RequestBody.Content.JSON = new(ContentType)
		path.Post.RequestBody.Content.JSON.Schema = sb.walkMethodArgs(info)
		path.Post.Responses.OK.Description = "Success"

		path.Post.Responses.OK.Content.JSON = new(ContentType)
		if !info.HasResponse() {
			path.Post.Responses.OK.Content.JSON.Schema = sb.defaults.Any
		} else {
			path.Post.Responses.OK.Content.JSON.Schema = sb.walk(info.Response())
		}

		path.Post.Responses.BadRequest = badRequest
		path.Post.Responses.InternalError = internalError
		schema.Paths["/"+strings.ToLower(method)] = path
	}

	schema.Components.Schemas = map[string]*Type{}
	for ref, component := range sb.components {
		if ref.name == "" {
			continue
		}
		schema.Components.Schemas[component.Name] = component
	}
	return &schema
}

// Title for schema.
func Title(title string) Option {
	return func(builder *schemaBuilder) {
		builder.title = title
	}
}

// Version of API.
func Version(version string) Option {
	return func(builder *schemaBuilder) {
		builder.version = version
	}
}

// Define specific type as OpenAPI definition.
func Define(pkg, name string, definition *Type) Option {
	return func(builder *schemaBuilder) {
		builder.hooks[schemaRef{
			pkg:  pkg,
			name: name,
		}] = definition
	}
}
