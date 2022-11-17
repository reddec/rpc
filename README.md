# RPC

[![Go Reference](https://pkg.go.dev/badge/github.com/reddec/rpc.svg)](https://pkg.go.dev/github.com/reddec/rpc)

> Calling Golang backend should be simple.

- no dependencies
- 100% test coverage
- simple and efficient

This project was created within one night after my frustration while writing yet another service which basically exposes
DB operations as an HTTP endpoint. The problem is that most of the current approaches offer some kind of framework lock.
Once you write your business logic, you will, basically, have to duplicate the method, which will just accept values
from an HTTP request and proxy it to the business function.

This project allows you to expose almost any kind of structure method as HTTP endpoints.

Supports:

- Any input and output arguments as soon as it is supported by JSON encoder/decoder
    - (optionally) First argument can be `context.Context` and it will be wired to `request.Context()`
- Value and/or error output. Example:
    - `Foo(...)`
    - `Foo(...) error`
    - `Foo(...) int64`
    - `Foo(...) (int64, error)`

Simplest possible example:

```go
package main

type Service struct{}

func (srv *Service) Sum(a, b int64) int64 {
	return a + b
}

func main() {
	http.Handle("/api/", http.StripPrefix("/api", rpc.New(&Service{})))
	http.ListenAndServe("127.0.0.1:8080", nil)
}
```

In JS side (you can just copy-and-paste)

```js
function RPC(baseURL = "") {
    return new Proxy({}, {
        get(obj, method) {
            method = method.toLowerCase();
            if (method in obj) {
                return obj[method]
            }

            const url = baseURL + "/" + encodeURIComponent(method)
            const fn = async function () {
                const args = Array.prototype.slice.call(arguments);
                const res = await fetch(url, {
                    method: "POST",
                    body: JSON.stringify(args),
                    headers: {
                        "Content-Type": "application/json"
                    }
                })
                if (!res.ok) {
                    const errMessage = await res.text();
                    throw new Error(errMessage);
                }
                return await res.json()
            }
            return obj[method] = fn
        }
    })
}
```

And use it as:

```js
const api = RPC("/api");

const amount = await api.sum(123, 456)
```

**Alternative** is to use CDN (seriously? for 375 bytes?)

```html

<script type="module">
    import RPC from "https://cdn.jsdelivr.net/gh/reddec/rpc@1/js/rpc.min.js"

    const API = RPC("/api");
    const total = await API.sum(123, 456);
</script>
```

## Dynamic session

In some cases you may need to prepare session, based on request: find user, authenticate it and so on. For that
use `Builder`.

`Builder` will invoke factory on each request and use returned value as API session object.

For example:

```go
package main

import (
	"net/http"
	"github.com/reddec/rpc"
)

type userSession struct {
	user string
}

func (us *userSession) Greet() string { // this will be an exported method
	return "Hello, " + us.user + "!"
}

type server struct{}

func (srv *server) newSession(r *http.Request) (*userSession, error) {
	user := r.Header.Get("X-User") // mimic real authorization
	return &userSession{
		user: user,
	}, nil
}

func main() {
	var srv server // initialize it!
	http.Handle("/api/", http.StripPrefix("/api", rpc.Builder(srv.newSession)))
	http.ListenAndServe("127.0.0.1:8080", nil)
}
```

Now, on call `api.greet()`, first will be executed `newSession` and then `userSession.Greet`

### Supporting tools


#### RPC script

Minified version of js/rpc.js supporting script (~400B) embedded to the library and available as global variable
`rpc.JS` and can be exposed as handler by `Script` function:

```go
// ...
http.Handle("/static/js/rpc.min.js", rpc.Script())
```

#### Schema

Package `schema` provides simple way to generate OpenAPI 3.1 schema based on indexed methods from server.
The generated object could be serialized as YAML or JSON and served as handler.

The function `schema.OpenAPI` uses result of `Index` function to generate schema and definition.

```go
var srv Server
index := rpc.Index(&srv)
schema := schema.OpenAPI(index) // customizable by Options
// render as JSON or YAML
```

For the convenience use handler to export schema over HTTP. It will pre-generate and cache schema.

```go
var srv Server
index := rpc.Index(&srv)
// ...
http.Handle("/schema", schema.Handler(index))
```