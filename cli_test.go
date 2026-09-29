package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	code = run(context.Background(), newServer(c, false), c, func() error { return nil }, args, &out, &errOut)
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
		case "/aktuelles/elternbriefe":
			io.WriteString(w, `<div id="asam_content"><table class="ui"><tr><td>#7</td><td id="empf_1">Empfang bestätigt.</td></tr><tr><td><a href="aktuelles/get_file/?repo=7" class="link_nachrichten dynamic-file"><h4>Brief</h4> 01.09.2026, 08:00</a><a href="aktuelles/get_file/?repo=7" class="dynamic-file">brief.pdf</a><span class="small">Klasse/n: 7C</span></td></tr></table></div>`)
		case "/aktuelles/get_file/":
			w.Header().Set("Content-Type", "application/pdf")
			io.WriteString(w, "%PDF-1.4 x")
		case "/meldungen/kommunikation_fachlehrer":
			io.WriteString(w, `<table id="messages-fachlehrer-table"><tbody></tbody></table>`)
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
	if code := run(context.Background(), newServer(c, false), c, ready, []string{"letters", "-h"}, &out, &errOut); code != 0 {
		t.Fatalf("help: exit %d, stderr %q", code, errOut.String())
	}
	if code := run(context.Background(), newServer(c, false), c, ready, []string{"letters"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "missing config") {
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

func TestGetLetterEmbedsFile(t *testing.T) {
	t.Setenv("PATH", "")
	cs := connect(t, fakePortal(t), false)
	call := func(include bool) *mcp.CallToolResult {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_letter", Arguments: map[string]any{"number": 7, "include_files": include}})
		if err != nil || res.IsError {
			t.Fatalf("call: %v %+v", err, res)
		}
		return res
	}
	var blob *mcp.ResourceContents
	var jsonText bool
	for _, c := range call(true).Content {
		switch c := c.(type) {
		case *mcp.EmbeddedResource:
			blob = c.Resource
		case *mcp.TextContent:
			jsonText = strings.Contains(c.Text, `"number":7`)
		}
	}
	if blob == nil || !strings.HasSuffix(blob.URI, "letter-7-brief.pdf") || !strings.HasPrefix(string(blob.Blob), "%PDF") || !jsonText {
		t.Fatalf("resource %+v, json text %v", blob, jsonText)
	}
	for _, c := range call(false).Content {
		if _, ok := c.(*mcp.EmbeddedResource); ok {
			t.Fatal("file embedded without include_files")
		}
	}
}

func TestDownloadCommand(t *testing.T) {
	t.Setenv("PATH", "")
	dir := t.TempDir()
	code, out, errOut := runCLI(t, fakePortal(t), "download", "--letter", "7", "--dir", dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	p := filepath.Join(dir, "letter-7-brief.pdf")
	if b, err := os.ReadFile(p); err != nil || !strings.HasPrefix(string(b), "%PDF") {
		t.Fatalf("file: %v", err)
	}
	if !strings.Contains(out, p) {
		t.Fatalf("stdout %q lacks %s", out, p)
	}
}

func TestDownloadNeedsOneTarget(t *testing.T) {
	code, _, errOut := runCLI(t, fakePortal(t), "download", "--letter", "7", "--thread-id", "1")
	if code != 1 || !strings.Contains(errOut, "exactly one") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestSyncCommand(t *testing.T) {
	t.Setenv("PATH", "")
	dir := t.TempDir()
	code, out, errOut := runCLI(t, fakePortal(t), "sync", "--dir", dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "letter-7-brief.pdf")); err != nil || !strings.Contains(out, `"saved"`) {
		t.Fatalf("stat %v, stdout %q", err, out)
	}
}
