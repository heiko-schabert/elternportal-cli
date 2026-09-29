package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"elternportal-cli/portal"

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
	want := []string{"check_login", "get_bulletin", "get_events", "get_exams", "get_letter", "get_message", "get_substitutions", "list_children", "list_letters", "list_messages", "list_teachers"}
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

func TestChildParamInSchema(t *testing.T) {
	cs := connect(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), false)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		if tl.Name != "get_letter" {
			continue
		}
		b, _ := json.Marshal(tl.InputSchema)
		for _, p := range []string{`"child"`, `"number"`} {
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
	if slices.Contains(names(false), "confirm_letter") {
		t.Fatal("write tool registered without allowWrite")
	}
	for _, n := range []string{"confirm_letter", "reply", "new_message", "contact_class_teacher"} {
		if slices.Contains(names(false), n) {
			t.Errorf("%s registered without allowWrite", n)
		}
		if !slices.Contains(names(true), n) {
			t.Errorf("%s missing with allowWrite", n)
		}
	}
}
