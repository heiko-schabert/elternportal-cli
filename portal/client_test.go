package portal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

const loginPage = `<html><form class="form-signin"><input type="hidden" name="csrf" value="tok"></form></html>`

// fakePortal mimics the portal's session handling: unknown or expired
// sessions get the login page with HTTP 200, like the real one.
type fakePortal struct {
	*httptest.Server
	mu        sync.Mutex
	valid     map[string]bool
	logins    int
	pages     map[string]string
	child     string
	hits      map[string]int
	lastQuery map[string]string
	onHit     func(path string)
	posts     map[string]url.Values
	onPost    func(path string, v map[string][]string)
}

func newFakePortal(t *testing.T, pages map[string]string) *fakePortal {
	t.Helper()
	f := &fakePortal{valid: map[string]bool{}, pages: pages, hits: map[string]int{}, lastQuery: map[string]string{}, posts: map[string]url.Values{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakePortal) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/includes/project/auth/login.php" {
		f.logins++
		if r.FormValue("csrf") != "tok" || r.FormValue("username") != "u" || r.FormValue("password") != "p" {
			io.WriteString(w, loginPage)
			return
		}
		sid := fmt.Sprint("s", f.logins)
		f.valid[sid] = true
		f.child = "1906"
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: sid, Path: "/"})
		http.Redirect(w, r, "/start", http.StatusFound)
		return
	}
	if ck, err := r.Cookie("sid"); err != nil || !f.valid[ck.Value] {
		io.WriteString(w, loginPage)
		return
	}
	f.hits[r.URL.Path]++
	f.lastQuery[r.URL.Path] = r.URL.RawQuery
	if f.onHit != nil {
		f.onHit(r.URL.Path)
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.posts[r.URL.Path] = r.MultipartForm.Value
		if f.onPost != nil {
			f.onPost(r.URL.Path, r.MultipartForm.Value)
		}
		http.Redirect(w, r, "/start", http.StatusFound)
		return
	}
	switch r.URL.Path {
	case "/api/set_child.php":
		f.child = r.URL.Query().Get("id")
		io.WriteString(w, "1")
		return
	case "/wer":
		io.WriteString(w, `<html><div id="asam_content">`+f.child+`</div></html>`)
		return
	}
	body, ok := f.pages[r.URL.Path]
	if !ok {
		body = kinderPage(f.child)
	}
	ct := "text/html; charset=utf-8"
	if strings.HasPrefix(body, "%PDF") {
		ct = "application/pdf"
	}
	w.Header().Set("Content-Type", ct)
	io.WriteString(w, body)
}

func kinderPage(selected string) string {
	opt := func(id, name string) string {
		sel := ""
		if id == selected {
			sel = " selected"
		}
		return `<option value="` + id + `"` + sel + `>` + name + `</option>`
	}
	return `<html><div class="pupil-selector"><select>` + opt("1906", "Anna Muster (7C)") + opt("1907", "Ben Muster (5A)") + `</select></div></html>`
}

func (f *fakePortal) expire() {
	f.mu.Lock()
	defer f.mu.Unlock()
	clear(f.valid)
}

func (f *fakePortal) client(password string) *Client {
	return New(Config{URL: f.URL, User: "u", Password: password})
}

func fixture(t *testing.T, name string) *goquery.Selection {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	s, err := content(b, "text/html; charset=utf-8", name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

const okPage = `<html><div id="asam_content"><p>Hallo</p></div></html>`

func TestPage(t *testing.T) {
	f := newFakePortal(t, map[string]string{"/x": okPage})
	s, err := f.client("p").page(context.Background(), "/x")
	if err != nil {
		t.Fatal(err)
	}
	if got := text(s); got != "Hallo" {
		t.Fatalf("got %q", got)
	}
}

func TestReloginOnExpiry(t *testing.T) {
	f := newFakePortal(t, map[string]string{"/x": okPage})
	c := f.client("p")
	ctx := context.Background()
	if _, err := c.page(ctx, "/x"); err != nil {
		t.Fatal(err)
	}
	f.expire()
	if _, err := c.page(ctx, "/x"); err != nil {
		t.Fatal(err)
	}
	if f.logins != 2 {
		t.Fatalf("logins = %d, want 2", f.logins)
	}
}

func TestBadPassword(t *testing.T) {
	f := newFakePortal(t, nil)
	err := f.client("falsch").CheckLogin(context.Background())
	if !errors.Is(err, ErrLogin) {
		t.Fatalf("err = %v, want ErrLogin", err)
	}
	if f.logins != 1 {
		t.Fatalf("logins = %d, want 1", f.logins)
	}
	if strings.Contains(err.Error(), "falsch") {
		t.Fatal("error leaks password")
	}
}

func TestMissingContent(t *testing.T) {
	f := newFakePortal(t, nil)
	_, err := f.client("p").page(context.Background(), "/leer")
	if err == nil || !strings.Contains(err.Error(), "/leer") {
		t.Fatalf("err = %v, want mention of /leer", err)
	}
}

func TestContentLatin1(t *testing.T) {
	b := []byte("<html><div id=\"asam_content\">Pr\xfcfung</div></html>")
	s, err := content(b, "text/html; charset=iso-8859-1", "x")
	if err != nil {
		t.Fatal(err)
	}
	if got := text(s); got != "Prüfung" {
		t.Fatalf("got %q", got)
	}
}

func TestText(t *testing.T) {
	b := []byte(`<div id="asam_content"><h4> A </h4><p>b<br>c</p>  </div>`)
	s, _ := content(b, "", "x")
	if got := text(s); got != "A\nb\nc" {
		t.Fatalf("got %q", got)
	}
}

func TestResolve(t *testing.T) {
	c := New(Config{URL: "https://x.eltern-portal.org"})
	for href, want := range map[string]string{
		"aktuelles/get_file/?repo=1":        "/aktuelles/get_file/?repo=1",
		"/aktuelles/get_file/?repo=1":       "/aktuelles/get_file/?repo=1",
		"https://x.eltern-portal.org/a?b=1": "/a?b=1",
	} {
		got, err := c.resolve(href)
		if err != nil || got != want {
			t.Errorf("resolve(%q) = %q, %v; want %q", href, got, err, want)
		}
	}
}

func TestContentModulAus(t *testing.T) {
	b, err := os.ReadFile("testdata/modul_aus.html")
	if err != nil {
		t.Fatal(err)
	}
	_, err = content(b, "text/html; charset=utf-8", "/service/vertretungsplan")
	if err == nil || !strings.Contains(err.Error(), "nicht aktiv") {
		t.Fatalf("err = %v, want module-inactive error", err)
	}
}
