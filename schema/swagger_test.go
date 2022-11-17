package schema_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/reddec/rpc"
	"github.com/reddec/rpc/schema"
)

type Account struct {
	Logo []byte
	Link struct {
		Domain string
		Path   string
	}
	Cents       uint
	SubAccounts []*Account
}

type User struct {
	Name     string `json:"user_name"`
	Year     uint16
	Password string `json:"-"`
	Address  struct {
		Country string
		City    string
		ZIP     int
	}
	Accounts   []*Account
	Primary    Account
	CreatedAt  time.Time
	TTL        time.Duration
	Registered bool
	Age        int8
	Status     byte
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
	var srv Server
	index := rpc.Index(&srv)
	schema := schema.OpenAPI(index, schema.Version("1.0.0"))
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf(buf.String())
}
