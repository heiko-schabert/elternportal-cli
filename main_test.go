package main

import (
	"context"
	"slices"
	"testing"

	"elternportal-mcp/portal"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T, c *portal.Client) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := newServer(c).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestTools(t *testing.T) {
	cs := connect(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}))
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tl := range res.Tools {
		got = append(got, tl.Name)
	}
	slices.Sort(got)
	want := []string{"check_login", "get_elternbrief", "get_schulaufgaben", "get_schwarzes_brett", "get_vertretungsplan", "list_elternbriefe"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestToolError(t *testing.T) {
	cs := connect(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}))
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "check_login", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("want IsError for unreachable portal")
	}
}
