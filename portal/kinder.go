package portal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Kind struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Klasse string `json:"klasse"`
}

type Kinder struct {
	Kinder []Kind `json:"kinder"`
}

var kindRe = regexp.MustCompile(`^(.*?)\s*\((\w+)\)$`)

func parseKinder(s *goquery.Selection) (kinder []Kind, selected string) {
	kinder = []Kind{}
	s.Find(".pupil-selector option").Each(func(_ int, o *goquery.Selection) {
		id, _ := o.Attr("value")
		k := Kind{ID: id, Name: norm(o.Text())}
		if m := kindRe.FindStringSubmatch(k.Name); m != nil {
			k.Name, k.Klasse = m[1], m[2]
		}
		kinder = append(kinder, k)
		if _, ok := o.Attr("selected"); ok {
			selected = id
		}
	})
	return kinder, selected
}

// pickKind refuses to guess between several children, so an agent never
// silently reads the wrong child's data.
func pickKind(kinder []Kind, name string) (Kind, error) {
	var names []string
	for _, k := range kinder {
		names = append(names, k.Name)
	}
	if name == "" {
		switch len(kinder) {
		case 0:
			return Kind{}, nil
		case 1:
			return kinder[0], nil
		}
		return Kind{}, fmt.Errorf("mehrere Kinder, kind angeben: %s", strings.Join(names, ", "))
	}
	for _, k := range kinder {
		first, _, _ := strings.Cut(k.Name, " ")
		if strings.EqualFold(name, first) || strings.EqualFold(name, k.Name) {
			return k, nil
		}
	}
	return Kind{}, fmt.Errorf("Kind %q nicht gefunden, vorhanden: %s", name, strings.Join(names, ", "))
}

func (c *Client) Kinder(ctx context.Context) (Kinder, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return Kinder{}, err
		}
	}
	return Kinder{Kinder: c.kinder}, nil
}

// UseKind selects a child and holds the selection until release; the portal
// stores it per session, so concurrent tool calls must not interleave.
func (c *Client) UseKind(ctx context.Context, name string) (func(), error) {
	c.opMu.Lock()
	if err := c.selectKind(ctx, name); err != nil {
		c.opMu.Unlock()
		return nil, err
	}
	return c.opMu.Unlock, nil
}

func (c *Client) selectKind(ctx context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return err
		}
	}
	k, err := pickKind(c.kinder, name)
	if err != nil {
		return err
	}
	c.want = k.ID
	if err := c.applyKind(ctx); !errors.Is(err, errExpired) {
		return err
	}
	// Session expired since the last request; login re-applies c.want.
	return c.login(ctx)
}

// errExpired reports that the portal answered with its login page.
var errExpired = errors.New("Session abgelaufen")

// applyKind restores the wanted child; each login resets it to the portal default.
func (c *Client) applyKind(ctx context.Context) error {
	if c.want == "" || c.want == c.selected {
		return nil
	}
	body, _, err := c.do(ctx, http.MethodPost, "/api/set_child.php?id="+url.QueryEscape(c.want), nil)
	if err != nil {
		return err
	}
	if isLoginPage(body) {
		return errExpired
	}
	if strings.TrimSpace(string(body)) != "1" {
		return fmt.Errorf("Kindwechsel zu %s fehlgeschlagen", c.want)
	}
	c.selected = c.want
	return nil
}
