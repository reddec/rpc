// Package jrpc defines JSON-oriented, simple version of HTTP RPC
package jrpc

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"reflect"
)

//go:embed index.html
var indexPage []byte

// New RPC exporter which scans object (usually pointer to struct) and indexes all public methods.
//
// Without payload
//
//	f()
//	f() -> v
//	f() -> error
//	f() -> (v, error)
//
// With context
//
//	f(ctx)
//	f(ctx) -> v
//	f(ctx) -> error
//	f(ctx) -> (v, error)
//
// With context and payload
//
//	f(ctx, payload)
//	f(ctx, payload) -> v
//	f(ctx, payload) -> error
//	f(ctx, payload) -> (v, error)
//
// With payload
//
//	f(payload)
//	f(payload) -> v
//	f(payload) -> error
//	f(payload) -> (v, error)
//
// See [RPC.ServeHTTP] for details.
func New(object any, options ...Option) *RPC {
	value := reflect.ValueOf(object)
	t := value.Type()
	errorInterface := reflect.TypeOf((*error)(nil)).Elem()
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	res := make(map[string]*exposedMethod)
	n := t.NumMethod()
	for i := 0; i < n; i++ {
		method := t.Method(i)

		args := method.Type.NumIn()
		out := method.Type.NumOut()

		// check output
		hasError := out > 0 && method.Type.Out(out-1).Implements(errorInterface)

		// too many returns
		if out > 2 {
			continue
		}
		// two returns and the last one is not an error
		if out == 2 && !hasError {
			continue
		}

		hasResponse := out == 1 && !hasError || out == 2

		// check input
		hasContext := args > 1 && method.Type.In(1).Implements(ctxInterface)
		hasArg := hasContext && args == 3 || !hasContext && args == 2

		// too many args
		if args > 3 { // receiver + ctx + arg
			continue
		}
		// for two args (+receiver), the first one must be context
		if !hasContext && args == 3 {
			continue
		}

		// build
		var responseType reflect.Type
		if hasResponse {
			responseType = method.Type.Out(0)
		}

		var argType reflect.Type
		if hasArg {
			argType = method.Type.In(args - 1)
		}

		em := &exposedMethod{
			hasContext:  hasContext,
			hasArg:      hasArg,
			hasError:    hasError,
			hasResponse: hasResponse,
			obj:         value,
			argType:     argType,
			retType:     responseType,
			method:      method,
		}

		handler := em
		res[method.Name] = handler
	}

	schema, err := json.Marshal(generateOpenAPI(res, options...))
	if err != nil {
		panic(err) // should never happen
	}
	return &RPC{
		schema:  schema,
		methods: res,
	}
}

type RPC struct {
	schema  []byte
	methods map[string]*exposedMethod
}

// ServeHTTP accepts POST request with JSON payload (Content-Type header is NOT checked).
//
// - only POST is allowed, otherwise 405 Method Not Allowed will be returned
// - in case of exported method is not accepting payload, payload will be ignored
// - in case of error during decoding payload, 400 Bad Request returned with plain text details
// - in case of unknown method (case-sensitive), 404 Not Found returned
// - in case of error during call, 500 Internal Server Error returned with plain text details
// - in case of exported method is not returning value, 204 No Content returned, otherwise 200 OK and JSON (with proper headers)
func (rpc *RPC) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	method := path.Base(request.URL.Path)
	if request.Method == http.MethodGet {
		if method == "" || method == "/" {
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write(indexPage)
			return
		}
		if method == "swagger.json" { // schema
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write(rpc.schema)
			return
		}
	}

	m, ok := rpc.methods[method]
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !ok {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	var input json.RawMessage
	if m.hasArg {
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			writer.Header().Set("Content-Type", "text/plain")
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(err.Error()))
			return
		}
	}

	output, err := m.call(request.Context(), input)
	if err != nil {
		writer.Header().Set("Content-Type", "text/plain")
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(err.Error()))
		return
	}

	if !m.hasResponse {
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(output)
}

type exposedMethod struct {
	hasContext  bool
	hasArg      bool
	hasError    bool
	hasResponse bool

	obj     reflect.Value
	argType reflect.Type
	retType reflect.Type
	method  reflect.Method
}

func (m *exposedMethod) call(ctx context.Context, data json.RawMessage) (json.RawMessage, error) {
	var args = make([]reflect.Value, 0, 3)
	args = append(args, m.obj)
	if m.hasContext {
		args = append(args, reflect.ValueOf(ctx))
	}
	if m.hasArg {
		v, err := m.parseArg(data)
		if err != nil {
			return nil, fmt.Errorf("parse: %w", err)
		}
		args = append(args, v)
	}
	output := m.method.Func.Call(args)
	responseValues := toAny(output)

	if m.hasError {
		if v := responseValues[len(responseValues)-1]; v != nil {
			return nil, v.(error)
		}
	}

	if !m.hasResponse {
		return nil, nil
	}

	res, err := json.Marshal(responseValues[0])
	if err != nil {
		return nil, fmt.Errorf("encode result: %w", err)
	}
	return res, nil
}

func (m *exposedMethod) parseArg(data json.RawMessage) (reflect.Value, error) {
	argValue := reflect.New(m.argType)
	if err := json.Unmarshal(data, argValue.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("decode payload: %w", err)
	}
	return argValue.Elem(), nil
}

func toAny(values []reflect.Value) []any {
	var ans = make([]any, len(values))
	for i, v := range values {
		ans[i] = v.Interface()
	}
	return ans
}
