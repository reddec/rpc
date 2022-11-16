package rpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/reddec/rpc"
)

type Account struct {
	Logo []byte
	Link struct {
		Domain string
		Path   string
	}
}

type User struct {
	Name     string `json:"user_name"`
	Year     uint16
	Password string `json:"-"`
	Address  struct {
		Country string
		City    string
	}
	Accounts  []*Account
	CreatedAt time.Time
}

type Server struct{}

func (srv *Server) Register(ctx context.Context, user *User) (int64, error) {
	return 0, nil
}

func (srv *Server) GetUser(ctx context.Context, id int64) (*User, error) {
	return nil, nil
}

func (srv *Server) RemoveAllUsers() {
}

func TestOpenAPI(t *testing.T) {
	schema := rpc.OpenAPI[Server]()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf(buf.String())
}
