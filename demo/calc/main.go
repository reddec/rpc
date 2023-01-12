package main

import (
	_ "embed"
	"flag"
	"fmt"
	"net/http"

	"github.com/reddec/rpc"
	"github.com/reddec/rpc/schema"
)

//go:embed index.html
var mainPage []byte

type calc struct {
}

func (c *calc) Sum(a, b float64) float64 {
	return a + b
}

const apiPrefix = "/api"

func main() {
	bind := flag.String("bind", "127.0.0.1:8080", "Binding address")
	flag.Parse()

	var service calc

	index := rpc.Index(&service)
	openapi := schema.OpenAPI(index,
		schema.Title("Demo API"),
		schema.Version("0.0.1"),
		schema.URL(apiPrefix),
	)

	http.Handle("/static/js/rpc.min.js", rpc.Script())
	http.Handle("/openapi", schema.Expose(openapi))
	http.Handle(apiPrefix+"/", http.StripPrefix(apiPrefix, rpc.Router(index)))
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(mainPage)
	})
	fmt.Println("http://" + *bind)
	_ = http.ListenAndServe(*bind, nil)
}
