package rpc

import (
	"math"
	"reflect"
	"strconv"
	"strings"
)

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
		OK Payload `json:"200" yaml:"200"`
		// TODO: non-200
	} `json:"responses" yaml:"responses"`
}

type Payload struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Content     struct {
		JSON struct {
			Schema *Type `json:"schema" yaml:"schema"`
		} `json:"application/json" yaml:"application/json"`
	} `json:"content" yaml:"content"`
}

type Type struct {
	Type        string           `json:"type,omitempty" yaml:"type,omitempty"`
	Format      string           `json:"format,omitempty" yaml:"format,omitempty"`
	Ref         string           `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Items       *Type            `json:"items,omitempty" yaml:"items,omitempty"`
	Properties  map[string]*Type `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string         `json:"required,omitempty" yaml:"required,omitempty"`
	Minimum     int64            `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum     int64            `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	PrefixItems []*Type          `json:"prefixItems,omitempty" yaml:"prefixItems,omitempty"`
	MinItems    int              `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	MaxItems    int              `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	Name        string           `json:"-" yaml:"-"`
}

// OpenAPI generates Open-API 3.1 schema. It's recommend to cache result.
func OpenAPI[T any]() *Schema {
	var instance = new(T)
	sb := schemaBuilder{
		components: make(map[schemaRef]*Type),
		names:      make(map[string]int),
	}
	methods := Index(instance)
	return sb.build(methods)
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

type schemaBuilder struct {
	components map[schemaRef]*Type
	names      map[string]int
}

func (sb *schemaBuilder) walk(t reflect.Type) *Type {
	switch t.Kind() {
	case reflect.Ptr:
		return sb.walk(t.Elem())
	case reflect.Int, reflect.Int64:
		return &Type{Type: "integer", Format: "int64"}
	case reflect.Int32:
		return &Type{Type: "integer", Format: "int32"}
	case reflect.Int16:
		return &Type{Type: "integer", Maximum: math.MaxInt16}
	case reflect.Int8:
		return &Type{Type: "integer", Maximum: math.MaxInt8}

	case reflect.Uint, reflect.Uint64:
		return &Type{Type: "integer", Format: "int64", Minimum: 0}
	case reflect.Uint32:
		return &Type{Type: "integer", Format: "int32", Minimum: 0}
	case reflect.Uint16:
		return &Type{Type: "integer", Minimum: 0, Maximum: math.MaxUint16}
	case reflect.Uint8:
		return &Type{Type: "integer", Minimum: 0, Maximum: math.MaxUint8}
	case reflect.String:
		return &Type{Type: "string"}
	case reflect.Bool:
		return &Type{Type: "boolean"}
	case reflect.Float32:
		return &Type{Type: "number", Format: "float"}
	case reflect.Float64:
		return &Type{Type: "number", Format: "double"}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			// base64
			return &Type{Type: "string", Format: "byte"}
		}
		return &Type{Type: "array", Items: sb.walk(t.Elem())}
	case reflect.Array:
		return &Type{Type: "array", Items: sb.walk(t.Elem()), MinItems: t.Len(), MaxItems: t.Len()}
	case reflect.Struct:
		switch {
		case t.PkgPath() == "time" && t.Name() == "Time":
			return &Type{Type: "string", Format: "date-time"}
		case t.Name() == "": //anonymous, we need to embed
			return sb.walkStruct(t)
		default:
			info := sb.walkStruct(t)
			return &Type{Ref: "#/components/schemas/" + info.Name}
		}
	default:
		return &Type{Type: "object"}
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

func (sb *schemaBuilder) walkMethodArgs(method *ExposedMethod) *Type {
	var res = &Type{
		Type:     "array",
		MinItems: len(method.argTypes),
		MaxItems: len(method.argTypes),
		Items:    &Type{},
	}
	for _, arg := range method.argTypes {
		res.PrefixItems = append(res.PrefixItems, sb.walk(arg))
	}
	return res
}

func (sb *schemaBuilder) build(index map[string]*ExposedMethod) *Schema {
	var schema = Schema{
		OpenAPI: "3.1.0",
		Paths:   map[string]Path{},
	}

	for method, info := range index {
		var path Path

		path.Post.OperationID = method
		path.Post.RequestBody.Content.JSON.Schema = sb.walkMethodArgs(info)
		path.Post.Responses.OK.Description = "Success"
		if !info.hasResponse {
			path.Post.Responses.OK.Content.JSON.Schema = &Type{}
		} else {
			path.Post.Responses.OK.Content.JSON.Schema = sb.walk(info.responseType)
		}
		// TODO: add negative code
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
