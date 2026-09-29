package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"elternportal-cli/portal"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func TestCommandNamesUnique(t *testing.T) {
	cs := connect(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), true)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, tl := range res.Tools {
		cmd := commandName(tl.Name)
		if prev, ok := seen[cmd]; ok {
			t.Errorf("%s and %s both map to %s", prev, tl.Name, cmd)
		}
		seen[cmd] = tl.Name
	}
	for cmd, tool := range map[string]string{"letter": "get_letter", "letters": "list_letters", "check-login": "check_login", "reply": "reply"} {
		if seen[cmd] != tool {
			t.Errorf("%s → %q, want %s", cmd, seen[cmd], tool)
		}
	}
}

func TestToolArgs(t *testing.T) {
	s := schema{
		Properties: map[string]property{
			"number":      {Type: "integer", Description: "Number"},
			"text":        {Type: "string"},
			"open_unread": {Type: "boolean"},
			"child":       {Type: "string"},
		},
		Required: []string{"number"},
	}
	cmd := &cobra.Command{Use: "x"}
	addFlags(cmd, s)
	if err := cmd.Flags().Parse([]string{"--number", "49", "--text", "49", "--open-unread"}); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(toolArgs(cmd.Flags(), s))
	// Unset flags stay out so the tool applies its own defaults.
	if string(b) != `{"number":49,"open_unread":true,"text":"49"}` {
		t.Fatalf("got %s", b)
	}
	if ann := cmd.Flags().Lookup("number").Annotations[cobra.BashCompOneRequiredFlag]; len(ann) == 0 {
		t.Error("number not marked required")
	}
	if u := cmd.Flags().Lookup("number").Usage; u != "Number" {
		t.Errorf("usage = %q", u)
	}
}

func runCLI(t *testing.T, c *portal.Client, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(context.Background(), newServer(c, false), func() error { return nil }, args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunHelp(t *testing.T) {
	code, out, _ := runCLI(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}))
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"letters", "check-login", "mcp", "ELTERNPORTAL_ALLOW_WRITE"} {
		if !strings.Contains(out, want) {
			t.Errorf("help lacks %q:\n%s", want, out)
		}
	}
}

func TestRunUnknownCommand(t *testing.T) {
	code, _, errOut := runCLI(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), "gibtsnicht")
	if code != 1 || !strings.Contains(errOut, "gibtsnicht") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestRunToolError(t *testing.T) {
	code, out, errOut := runCLI(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), "check-login")
	if code != 1 || out != "" || errOut == "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

// fakePortal serves just enough of the portal for one logged-in page.
func fakePortal(t *testing.T) *portal.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			io.WriteString(w, `<form class="form-signin"><input name="csrf" value="tok"></form>`)
		case "/includes/project/auth/login.php":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "1", Path: "/"})
			http.Redirect(w, r, "/start", http.StatusFound)
		case "/aktuelles/schwarzes_brett":
			io.WriteString(w, `<div id="asam_content"><div class="card"><div class="card-body"><h4>Mensa</h4><p>Neu.</p></div></div></div>`)
		default:
			io.WriteString(w, "<html>start</html>")
		}
	}))
	t.Cleanup(srv.Close)
	return portal.New(portal.Config{URL: srv.URL, User: "u", Password: "p"})
}

func TestRunCall(t *testing.T) {
	code, out, errOut := runCLI(t, fakePortal(t), "bulletin")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	var got portal.Bulletin
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, out)
	}
	if len(got.Notices) != 1 || got.Notices[0].Title != "Mensa" {
		t.Fatalf("got %+v", got)
	}
}

func TestCommandHelp(t *testing.T) {
	code, out, _ := runCLI(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), "letter", "-h")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"--number", "letter number", "--child"} {
		if !strings.Contains(out, want) {
			t.Errorf("help lacks %q:\n%s", want, out)
		}
	}
}

func TestRequiredFlag(t *testing.T) {
	code, _, errOut := runCLI(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), "message")
	if code != 1 || !strings.Contains(errOut, "thread-id") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestBadFlagValue(t *testing.T) {
	code, _, errOut := runCLI(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}), "letter", "--number", "x")
	if code != 1 || !strings.Contains(errOut, "invalid argument") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestConfigErrorOnlyOnCall(t *testing.T) {
	ready := func() error { return errors.New("missing config") }
	var out, errOut bytes.Buffer
	c := portal.New(portal.Config{})
	if code := run(context.Background(), newServer(c, false), ready, []string{"letters", "-h"}, &out, &errOut); code != 0 {
		t.Fatalf("help: exit %d, stderr %q", code, errOut.String())
	}
	if code := run(context.Background(), newServer(c, false), ready, []string{"letters"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "missing config") {
		t.Fatalf("call: exit %d, stderr %q", code, errOut.String())
	}
}

func TestMCPOverHTTP(t *testing.T) {
	srv := httptest.NewServer(httpHandler(newServer(portal.New(portal.Config{URL: "http://127.0.0.1:1"}), false)))
	t.Cleanup(srv.Close)
	ctx := context.Background()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) == 0 {
		t.Fatal("no tools over HTTP")
	}
}
