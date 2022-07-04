package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// Index object's (usually pointer to struct) method. Matched public methods will be wrapped to http handler, which
// parses request body as JSON array and passes it to function. Result will be returned also as json.
//
// Criteria for matching methods: no return values, or single return value/error, or two return values, where second one
// must be an error. First input argument could be context.Context which will be automatically wired from request.Context().
//
//     Foo()                                          // OK
//     Foo(ctx context.Context)                       // OK
//     Foo(ctx context.Context, bar int, baz SomeObj) // OK
//     Foo(bar int, baz string)                       // OK
//
//     Foo(...) error        // OK
//     Foo(...) int          // OK
//     Foo(...) (int, error) // OK
//     Foo(...) (int, int)   // NOT ok - last argument is not an error
//
// Handler will return
//
// 400 Bad Request in case payload can not be unmarshalled to arguments or number of arguments not enough.
//
// 500 Internal Server Error in case method returned an error. Response payload will be error message (plain text)
//
// 200 OK in case everything fine
func Index(object interface{}) map[string]*ExposedMethod {
	value := reflect.ValueOf(object)
	t := value.Type()
	errorInterface := reflect.TypeOf((*error)(nil)).Elem()
	ctxInterface := reflect.TypeOf((*context.Context)(nil)).Elem()

	res := make(map[string]*ExposedMethod)
	n := t.NumMethod()

	for i := 0; i < n; i++ {
		method := t.Method(i)

		args := method.Type.NumIn()
		out := method.Type.NumOut()
		hasError := out > 0 && method.Type.Out(out-1).Implements(errorInterface)

		if out > 2 || (out == 2 && !hasError) {
			continue
		}

		hasResponse := out == 1 && !hasError || out == 2

		hasContext := args > 1 && method.Type.In(1).Implements(ctxInterface)

		var offset = 1 // first arg is receiver
		if hasContext {
			offset++
		}

		var argTypes []reflect.Type
		for arg := offset; arg < args; arg++ {
			argTypes = append(argTypes, method.Type.In(arg))
		}

		em := &ExposedMethod{
			args:        args,
			receiver:    value,
			argTypes:    argTypes,
			hasResponse: hasResponse,
			hasContext:  hasContext,
			hasError:    hasError,
			offset:      offset,
			method:      method,
		}

		handler := em
		res[method.Name] = handler
	}
	return res
}

type ExposedMethod struct {
	args        int
	receiver    reflect.Value
	argTypes    []reflect.Type
	hasResponse bool
	hasContext  bool
	hasError    bool
	offset      int
	method      reflect.Method
}

func (em *ExposedMethod) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	em.invoke(em.receiver, writer, request)
}

func (em *ExposedMethod) invoke(receiver reflect.Value, writer http.ResponseWriter, request *http.Request) {
	var argValues = make([]reflect.Value, em.offset+len(em.argTypes))
	argValues[0] = receiver
	if em.hasContext {
		argValues[1] = reflect.ValueOf(request.Context())
	}
	dataArgs := argValues[em.offset:]

	var params []json.RawMessage

	if err := json.NewDecoder(request.Body).Decode(&params); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	if len(params) < len(dataArgs) {
		http.Error(writer, "not enough arguments, expected "+strconv.Itoa(len(dataArgs)), http.StatusBadRequest)
		return
	}

	for arg := range dataArgs {
		argType := em.argTypes[arg]
		argValue := reflect.New(argType)
		if err := json.Unmarshal(params[arg], argValue.Interface()); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		dataArgs[arg] = argValue.Elem()
	}

	output := em.method.Func.Call(argValues)
	responseValues := toAny(output)

	var appError error

	if em.hasError {
		if v := responseValues[len(responseValues)-1]; v != nil {
			appError = v.(error)
		}
		responseValues = responseValues[:len(responseValues)-1]
	}

	var response any
	if em.hasResponse {
		response = responseValues[0]
	}

	if appError != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(appError.Error()))
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	var encoder = json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(response) // too late to do anything
}

// Router creates mux handler which exposes all indexed method with name as path, in lower case,
// and only for POST method.
//
//     http.Handle("/api/", http.StripPrefix("/api", Router(...)))
//
//     MyFoo(..) -> POST /myfoo
//
func Router(index map[string]*ExposedMethod) http.Handler {
	mux := http.NewServeMux()
	for name, handler := range index {
		mux.Handle("/"+strings.ToLower(name), handler)
	}

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "only POST supported", http.StatusMethodNotAllowed)
			return
		}
		mux.ServeHTTP(writer, request)
	})
}

// Builder creates new path-based, POST-only router, with custom receiver (aka session) for each request.
//
//     type API struct {
//         User string // to be filled by Server
//     }
//     type Server struct {}
//     func (srv *Server) newAPI(r *http.Request) (*API, error) {}
//
//     // ...
//     var server Server
//     handler := Builder(server.newAPI)
//
// Handler will return
//
// 400 Bad Request in case payload can not be unmarshalled to arguments or number of arguments not enough.
//
// 404 Not Found in case method is not known (case-insensitive).
//
// 500 Internal Server Error in case method returned an error or factory returned error. Response payload will be error message (plain text)
//
// 200 OK in case everything fine
func Builder[T any](factory func(r *http.Request) (T, error)) http.Handler {
	var t T
	handlers := Index(t)
	var caseHandlers = make(map[string]*ExposedMethod, len(handlers))
	for name, handler := range handlers {
		caseHandlers[strings.ToLower(name)] = handler
	}

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		method := strings.ToLower(strings.TrimPrefix(request.URL.Path, "/"))
		handler, ok := caseHandlers[method]
		if !ok {
			writer.WriteHeader(http.StatusNotFound)
			return
		}

		value, err := factory(request)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte(err.Error()))
			return
		}

		receiver := reflect.ValueOf(value)
		handler.invoke(receiver, writer, request)
	})
}

// New exposes matched methods of object as HTTP endpoints.
// It's shorthand for Router(Index(object)).
func New(object interface{}) http.Handler {
	return Router(Index(object))
}

func toAny(values []reflect.Value) []any {
	var ans = make([]any, len(values))
	for i, v := range values {
		ans[i] = v.Interface()
	}
	return ans
}
