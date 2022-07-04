package rpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/reddec/rpc"
)

type SomeObj struct {
	Hello string
}

type api struct {
	t       *testing.T
	reached string
}

func (api *api) Nothing() {
	api.reached = "Nothing"
}

func (api *api) Foo0(ctx context.Context) {
	api.reached = "Foo0"
	if ctx == nil {
		api.t.Error("empty context")
	}
}

func (api *api) Foo1(ctx context.Context, bar int, baz SomeObj) {
	api.reached = "Foo1"
	if ctx == nil {
		api.t.Error("empty context")
	}
	if bar != 123 {
		api.t.Error("invalid bar")
	}
	if baz.Hello != "hello" {
		api.t.Error("invalid baz")
	}
}

func (api *api) Foo2(bar int, baz string) {
	api.reached = "Foo2"

	if bar != 123 {
		api.t.Error("invalid bar")
	}
	if baz != "hello" {
		api.t.Error("invalid baz")
	}
}

func (api *api) Foo3() error {
	api.reached = "Foo3"
	return nil
}

func (api *api) Foo4() int {
	api.reached = "Foo4"
	return 123
}

func (api *api) Foo5() (int, error) {
	api.reached = "Foo5"
	return 0, nil
}

func (api *api) Skipped() (int, int) {
	return 0, 0
}

func (api *api) unexported() {

}

func (api *api) Calc(a, b int) int {
	api.reached = "Calc"
	return a + b
}

func (api *api) Fail() error {
	api.reached = "Fail"
	return errors.New("fail")
}

func TestIndex(t *testing.T) {
	t.Run("skip wrong", func(t *testing.T) {
		r := &api{t: t}
		index := rpc.Index(r)
		if _, ok := index["Skipped"]; ok {
			t.Error("should skip wrong method")
		}
	})
	t.Run("skip unexported", func(t *testing.T) {
		r := &api{t: t}
		index := rpc.Index(r)
		if _, ok := index["unexported"]; ok {
			t.Error("should skip unexported method")
		}
	})
	t.Run("call no args no return", testReach("Nothing"))
	t.Run("call with context", testReach("Foo0"))
	t.Run("call with context and args", testReach("Foo1", 123, SomeObj{Hello: "hello"}))
	t.Run("call only with args", testReach("Foo2", 123, "hello"))
	t.Run("call method which returns error obj", testReach("Foo3"))
	t.Run("call method which returns value", testReach("Foo4"))
	t.Run("call method which returns value and error obj", testReach("Foo5"))
	t.Run("call and check result", func(t *testing.T) {
		const method = "Calc"
		r := &api{t: t}
		index := rpc.Index(r)
		handler, ok := index[method]
		if !ok {
			t.Fatal("method should exists")
		}

		payload, err := json.Marshal([]interface{}{1, 2})
		if err != nil {
			t.Fatal(err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Error(rec.Code, rec.Body.String())
		}
		if r.reached != method {
			t.Error("not reached method")
		}
		var result int
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Error(err)
		}
		if result != 3 {
			t.Error("calculation miss match")
		}
	})
	t.Run("invalid arguments - not json", func(t *testing.T) {
		const method = "Calc"
		r := &api{t: t}
		index := rpc.Index(r)
		handler, ok := index[method]
		if !ok {
			t.Fatal("method should exists")
		}

		payload := []byte("abcd,123")

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Error("should be 400 bad request", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid arguments - different types", func(t *testing.T) {
		const method = "Calc"
		r := &api{t: t}
		index := rpc.Index(r)
		handler, ok := index[method]
		if !ok {
			t.Fatal("method should exists")
		}

		payload, err := json.Marshal([]interface{}{1, "2"})
		if err != nil {
			t.Fatal(err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Error("should be 400 bad request", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid arguments - not enough", func(t *testing.T) {
		const method = "Calc"
		r := &api{t: t}
		index := rpc.Index(r)
		handler, ok := index[method]
		if !ok {
			t.Fatal("method should exists")
		}

		payload, err := json.Marshal([]interface{}{1})
		if err != nil {
			t.Fatal(err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Error("should be 400 bad request", rec.Code, rec.Body.String())
		}
	})
	t.Run("treat error as 500", func(t *testing.T) {
		const method = "Fail"
		r := &api{t: t}
		index := rpc.Index(r)
		handler, ok := index[method]
		if !ok {
			t.Fatal("method should exists")
		}

		payload, err := json.Marshal([]interface{}{1})
		if err != nil {
			t.Fatal(err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Error("should be 500", rec.Code, rec.Body.String())
		}
		if r.reached != method {
			t.Error("not reached method")
		}
	})
}

func testReach(method string, args ...interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		r := &api{t: t}
		index := rpc.Index(r)
		handler, ok := index[method]
		if !ok {
			t.Fatal("method should exists")
		}

		payload, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Error(rec.Code, rec.Body.String())
		}
		if r.reached != method {
			t.Error("not reached method")
		}
	}
}

func TestRouter(t *testing.T) {
	r := &api{t: t}
	router := rpc.New(r)

	t.Run("plain call should work", func(t *testing.T) {
		payload, err := json.Marshal([]interface{}{1, 2})
		if err != nil {
			t.Fatal(err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/calc", bytes.NewReader(payload))
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Error(rec.Code, rec.Body.String())
		}
		var result int
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Error(err)
		}
		if result != 3 {
			t.Error("calculation miss match")
		}
	})

	t.Run("non-POST method should be blocked", func(t *testing.T) {
		payload, err := json.Marshal([]interface{}{1, 2})
		if err != nil {
			t.Fatal(err)
		}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/Calc", bytes.NewReader(payload))
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Error("should be not allowed", rec.Code, rec.Body.String())
		}
	})
}

// semi-realistic example to check concept

type userSession struct {
	user    string
	request int64
}

func (us *userSession) Greet() string {
	return fmt.Sprint("Hello, ", us.user, "! Your request is ", us.request)
}

type server struct {
	requestID int64
}

func (srv *server) newSession(r *http.Request) (*userSession, error) {
	user := r.Header.Get("X-User")
	if user == "fail" {
		return nil, fmt.Errorf("failed")
	}
	return &userSession{
		user:    user,
		request: atomic.AddInt64(&srv.requestID, 1),
	}, nil
}

func TestBuilder(t *testing.T) {
	t.Run("straightforward call should work", func(t *testing.T) {
		var srv server
		handler := rpc.Builder(srv.newSession)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/Greet", bytes.NewBufferString("[]"))
		req.Header.Set("X-User", "reddec")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Error(rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != `"Hello, reddec! Your request is 1"` {
			t.Error(rec.Body.String())
		}
	})
	t.Run("case doesn't matter", func(t *testing.T) {
		var srv server
		handler := rpc.Builder(srv.newSession)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/gReEt", bytes.NewBufferString("[]"))
		req.Header.Set("X-User", "reddec")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Error(rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != `"Hello, reddec! Your request is 1"` {
			t.Error(rec.Body.String())
		}
	})
	t.Run("unknown method should return 404", func(t *testing.T) {
		var srv server
		handler := rpc.Builder(srv.newSession)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/Unknown", bytes.NewBufferString("[]"))
		req.Header.Set("X-User", "reddec")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Error(rec.Code)
		}
	})
	t.Run("only post is allowed", func(t *testing.T) {
		var srv server
		handler := rpc.Builder(srv.newSession)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/Greet", bytes.NewBufferString("[]"))
		req.Header.Set("X-User", "reddec")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Error(rec.Code)
		}
	})
	t.Run("return 500 if session failed", func(t *testing.T) {
		var srv server
		handler := rpc.Builder(srv.newSession)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/Greet", bytes.NewBufferString("[]"))
		req.Header.Set("X-User", "fail")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Error(rec.Code)
		}
		if rec.Body.String() != "failed" {
			t.Error(rec.Body.String())
		}
	})
}
