package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/reddec/rpc"
	"github.com/reddec/rpc/schema"
)

//go:embed index.html
var mainPage []byte

type Encoder int

type SomeType struct {
	Name     string
	Age      int    `json:"the_age"`
	Enabled  bool   `json:",omitempty"`
	Password string `json:"-"`
	Ref      *int
	Location time.Location
	Document json.RawMessage
}

// Calc is API server.
// Multiple line
// docs are
// also supported
//
//go:generate go run github.com/reddec/rpc/cmd/rpc-ts@latest -shim time.Location:string
type Calc struct {
	name string
}

// Name of the person
func (c *Calc) Name(prefix string) calc {
	return calc{}
}

func (c *Calc) Today() time.Time {
	return time.Now()
}

// Update something
func (c *Calc) Update(tp SomeType) {

}

func (c *Calc) Binary() []byte {
	return []byte(c.name)
}

func (c *Calc) Bool() bool {
	return false
}

func (c *Calc) Custom() xml.Encoder {
	return xml.Encoder{}
}

func (c *Calc) Nillable() *time.Time {
	t := time.Now()
	return &t
}

func (c *Calc) Error() error {
	return nil
}

func (c *Calc) AnotherTime() Encoder {
	return 0
}

func (c *Calc) NillableSlice(enc Encoder) *[]*time.Time {
	return nil
}

func (c *Calc) AnonType() struct{ X int } {
	return struct{ X int }{X: 1}
}

func (c *Calc) Multiple(ctx context.Context, name string, a, b float64, ts time.Time) (bool, error) {
	return false, nil
}

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
