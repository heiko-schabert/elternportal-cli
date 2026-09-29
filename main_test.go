package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"elternportal-mcp/portal"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T, c *portal.Client, allowWrite bool) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := newServer(c, allowWrite).Connect(ctx, st, nil); err != nil {
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
	cs := connect(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), false)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tl := range res.Tools {
		got = append(got, tl.Name)
	}
	slices.Sort(got)
	want := []string{"check_login", "get_elternbrief", "get_nachricht", "get_schulaufgaben", "get_schwarzes_brett", "get_termine", "get_vertretungsplan", "list_elternbriefe", "list_kinder", "list_lehrkraefte", "list_nachrichten"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestToolError(t *testing.T) {
	cs := connect(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), false)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "check_login", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("want IsError for unreachable portal")
	}
}

func TestKindParamInSchema(t *testing.T) {
	cs := connect(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), false)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		if tl.Name != "get_elternbrief" {
			continue
		}
		b, _ := json.Marshal(tl.InputSchema)
		for _, p := range []string{`"kind"`, `"nummer"`} {
			if !strings.Contains(string(b), p) {
				t.Errorf("schema %s lacks %s", b, p)
			}
		}
	}
}

func TestWriteToolsGated(t *testing.T) {
	names := func(allow bool) []string {
		res, err := connect(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), allow).ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		var n []string
		for _, tl := range res.Tools {
			n = append(n, tl.Name)
		}
		return n
	}
	if slices.Contains(names(false), "elternbrief_bestaetigen") {
		t.Fatal("write tool registered without allowWrite")
	}
	if !slices.Contains(names(true), "elternbrief_bestaetigen") {
		t.Fatal("write tool missing with allowWrite")
	}
}
