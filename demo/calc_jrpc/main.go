package main

import (
	_ "embed"
	"flag"
	"fmt"
	"net/http"

	"github.com/reddec/rpc/jrpc"
)

type calc struct {
}

type Op struct {
	A float64
	B float64
}

func (c *calc) Sum(op Op) float64 {
	return op.A + op.B
}

func main() {
	bind := flag.String("bind", "127.0.0.1:8080", "Binding address")
	flag.Parse()

	var service calc
	http.Handle("/", jrpc.New(&service))
	fmt.Println("http://" + *bind)
	_ = http.ListenAndServe(*bind, nil)
}
