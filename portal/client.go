package portal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// ErrLogin means the portal rejected the credentials.
var ErrLogin = errors.New("Login fehlgeschlagen, Zugangsdaten prüfen")

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// Client is one portal session. The portal keeps state such as the selected
// child server-side, so requests are serialized.
type Client struct {
	cfg      Config
	http     *http.Client
	mu       sync.Mutex
	loggedIn bool
	opMu     sync.Mutex // spans child selection plus the requests using it
	kinder   []Kind
	selected string // child active in the portal session
	want     string // child the current operation needs
}

func New(cfg Config) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{cfg: cfg, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
}

func (c *Client) CheckLogin(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.login(ctx)
}

func (c *Client) login(ctx context.Context) error {
	c.loggedIn = false
	body, ct, err := c.do(ctx, http.MethodGet, "/", nil)
	if err != nil {
		return err
	}
	doc, err := parseHTML(body, ct)
	if err != nil {
		return err
	}
	csrf, ok := doc.Find(`input[name="csrf"]`).Attr("value")
	if !ok {
		return errors.New("Login-Seite: CSRF-Token fehlt")
	}
	body, ct, err = c.do(ctx, http.MethodPost, "/includes/project/auth/login.php", url.Values{
		"csrf": {csrf}, "username": {c.cfg.User}, "password": {c.cfg.Password}, "go_to": {""},
	})
	if err != nil {
		return err
	}
	if isLoginPage(body) {
		return ErrLogin
	}
	doc, err = parseHTML(body, ct)
	if err != nil {
		return err
	}
	c.kinder, c.selected = parseKinder(doc.Selection)
	c.loggedIn = true
	return c.applyKind(ctx)
}

// isLoginPage detects the login form; the portal answers expired sessions
// with it instead of an HTTP error.
func isLoginPage(body []byte) bool {
	return bytes.Contains(body, []byte("form-signin"))
}

// do sends one request; a non-nil form makes it a urlencoded POST.
func (c *Client) do(ctx context.Context, method, path string, form url.Values) ([]byte, string, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.URL+path, body)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", c.cfg.URL)
		req.Header.Set("Referer", c.cfg.URL+"/")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%s %s: %s", method, path, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	return b, resp.Header.Get("Content-Type"), err
}

// fetch GETs path with a valid session, re-logging in once if it expired.
func (c *Client) fetch(ctx context.Context, path string) ([]byte, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return nil, "", err
		}
	}
	b, ct, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil || !isLoginPage(b) {
		return b, ct, err
	}
	if err := c.login(ctx); err != nil {
		return nil, "", err
	}
	b, ct, err = c.do(ctx, http.MethodGet, path, nil)
	if err == nil && isLoginPage(b) {
		return nil, "", fmt.Errorf("%s: Session nach Re-Login ungültig", path)
	}
	return b, ct, err
}

func (c *Client) page(ctx context.Context, path string) (*goquery.Selection, error) {
	b, ct, err := c.fetch(ctx, path)
	if err != nil {
		return nil, err
	}
	return content(b, ct, path)
}

func (c *Client) resolve(href string) (string, error) {
	base, err := url.Parse(c.cfg.URL + "/")
	if err != nil {
		return "", err
	}
	u, err := base.Parse(href)
	if err != nil {
		return "", err
	}
	return u.RequestURI(), nil
}

func parseHTML(b []byte, contentType string) (*goquery.Document, error) {
	r, err := charset.NewReader(bytes.NewReader(b), contentType)
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(r)
}

// content returns #asam_content, the portal's main area; its absence means
// the layout changed and parsers would silently return nothing.
func content(b []byte, contentType, path string) (*goquery.Selection, error) {
	doc, err := parseHTML(b, contentType)
	if err != nil {
		return nil, err
	}
	s := doc.Find("#asam_content")
	if s.Length() == 0 {
		return nil, fmt.Errorf("%s: #asam_content fehlt, Portal-Layout geändert?", path)
	}
	// Schools can disable modules; the page then renders an empty frame.
	if s.Children().Length() == 0 && strings.TrimSpace(s.Text()) == "" {
		return nil, fmt.Errorf("%s: Seite leer, Modul an dieser Schule nicht aktiv", path)
	}
	return s, nil
}

// text joins non-empty text nodes line by line; goquery's Text() glues
// block elements together.
func text(s *goquery.Selection) string {
	var lines []string
	for _, n := range s.Nodes {
		for d := range n.Descendants() {
			if d.Type == html.TextNode {
				if t := strings.TrimSpace(d.Data); t != "" {
					lines = append(lines, t)
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

// doc returns a whole page; the messaging pages lack #asam_content.
func (c *Client) doc(ctx context.Context, path string) (*goquery.Document, error) {
	b, ct, err := c.fetch(ctx, path)
	if err != nil {
		return nil, err
	}
	return parseHTML(b, ct)
}

// download returns an attachment as text.
func (c *Client) download(ctx context.Context, href string) (string, error) {
	path, err := c.resolve(href)
	if err != nil {
		return "", err
	}
	raw, ct, err := c.fetch(ctx, path)
	if err != nil {
		return "", err
	}
	if strings.Contains(ct, "html") {
		return "", fmt.Errorf("%s: Download lieferte HTML statt Datei", path)
	}
	txt, err := pdfText(ctx, raw)
	if errors.Is(err, exec.ErrNotFound) {
		return "(pdftotext nicht installiert, PDF-Text nicht verfügbar)", nil
	}
	return txt, err
}
