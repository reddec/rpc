package jrpc

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type Calc struct{}

func (c *Calc) Sum(value []int) int {
	var a int
	for _, v := range value {
		a += v
	}
	return a
}

func (c *Calc) SumCtx(_ context.Context, value []int) int {
	var a int
	for _, v := range value {
		a += v
	}
	return a
}

func (c *Calc) Hi() string {
	return "hello"
}

func (c *Calc) Noop() {

}

func (c *Calc) Greet(name struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}) string {
	return "Hello, " + name.Prefix + " " + name.Name + "!"
}

func TestNew(t *testing.T) {
	r, err := New(&Calc{})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("sum", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/Sum", bytes.NewBufferString("[1,2,3]"))
		res := httptest.NewRecorder()

		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatal(res.Code, res.Body.String())
		}

		if res.Body.String() != "6" {
			t.Fatal(res.Body.String())
		}
	})

	t.Run("sum context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/SumCtx", bytes.NewBufferString("[1,2,3]"))
		res := httptest.NewRecorder()

		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatal(res.Code, res.Body.String())
		}

		if res.Body.String() != "6" {
			t.Fatal(res.Body.String())
		}
	})

	t.Run("hi", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/Hi", bytes.NewBufferString("[1,2,3]"))
		res := httptest.NewRecorder()

		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatal(res.Code, res.Body.String())
		}

		if res.Body.String() != `"hello"` {
			t.Fatal(res.Body.String())
		}
	})

	t.Run("noop", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/Noop", bytes.NewBufferString("[1,2,3]"))
		res := httptest.NewRecorder()

		r.ServeHTTP(res, req)
		if res.Code != http.StatusNoContent {
			t.Fatal(res.Code, res.Body.String())
		}
	})

	t.Run("landing page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		res := httptest.NewRecorder()

		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatal(res.Code, res.Body.String())
		}

		if res.Body.Len() < 5 {
			t.Fatal(res.Body.String())
		}

		if h := res.Header().Get("Content-Type"); h != "text/html" {
			t.Fatal(h)
		}
	})

	t.Run("swagger", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/swagger.json", nil)
		res := httptest.NewRecorder()

		r.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatal(res.Code, res.Body.String())
		}

		if res.Body.Len() < 5 {
			t.Fatal(res.Body.String())
		}

		if h := res.Header().Get("Content-Type"); h != "application/json" {
			t.Fatal(h)
		}
		t.Log(res.Body.String())
	})
}
