package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"elternportal-cli/portal"
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
	for cmd, tool := range map[string]string{"elternbrief": "get_elternbrief", "elternbriefe": "list_elternbriefe", "check-login": "check_login", "send-nachricht": "send_nachricht"} {
		if seen[cmd] != tool {
			t.Errorf("%s → %q, want %s", cmd, seen[cmd], tool)
		}
	}
}

func TestParseArgs(t *testing.T) {
	schema := map[string]any{"properties": map[string]any{
		"nummer":            map[string]any{"type": "integer"},
		"text":              map[string]any{"type": "string"},
		"ungelesen_oeffnen": map[string]any{"type": "boolean"},
	}}
	got, err := parseArgs([]string{"nummer=49", "text=49", "ungelesen-oeffnen=true"}, schema)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(got)
	if string(b) != `{"nummer":49,"text":"49","ungelesen_oeffnen":true}` {
		t.Fatalf("got %s", b)
	}
	for _, bad := range [][]string{{"nummer"}, {"nummer=x"}, {"unbekannt=1"}} {
		if _, err := parseArgs(bad, schema); err == nil {
			t.Errorf("%v: want error", bad)
		}
	}
}

func runCLI(t *testing.T, c *portal.Client, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(context.Background(), newServer(c, false), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunHelp(t *testing.T) {
	code, out, _ := runCLI(t, portal.New(portal.Config{URL: "http://127.0.0.1:1"}))
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"elternbriefe", "check-login", "mcp", "ELTERNPORTAL_ALLOW_WRITE"} {
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
	code, out, errOut := runCLI(t, fakePortal(t), "schwarzes-brett")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	var got portal.SchwarzesBrett
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, out)
	}
	if len(got.Aushaenge) != 1 || got.Aushaenge[0].Titel != "Mensa" {
		t.Fatalf("got %+v", got)
	}
}
