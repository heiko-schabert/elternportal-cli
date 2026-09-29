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

type Child struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

type Children struct {
	Children []Child `json:"children"`
}

var childRe = regexp.MustCompile(`^(.*?)\s*\((\w+)\)$`)

func parseChildren(s *goquery.Selection) (children []Child, selected string) {
	children = []Child{}
	s.Find(".pupil-selector option").Each(func(_ int, o *goquery.Selection) {
		id, _ := o.Attr("value")
		k := Child{ID: id, Name: norm(o.Text())}
		if m := childRe.FindStringSubmatch(k.Name); m != nil {
			k.Name, k.Class = m[1], m[2]
		}
		children = append(children, k)
		if _, ok := o.Attr("selected"); ok {
			selected = id
		}
	})
	return children, selected
}

// pickChild refuses to guess between several children, so an agent never
// silently reads the wrong child's data.
func pickChild(children []Child, name string) (Child, error) {
	var names []string
	for _, k := range children {
		names = append(names, k.Name)
	}
	if name == "" {
		switch len(children) {
		case 0:
			return Child{}, nil
		case 1:
			return children[0], nil
		}
		return Child{}, fmt.Errorf("several children, pass child: %s", strings.Join(names, ", "))
	}
	for _, k := range children {
		first, _, _ := strings.Cut(k.Name, " ")
		if strings.EqualFold(name, first) || strings.EqualFold(name, k.Name) {
			return k, nil
		}
	}
	return Child{}, fmt.Errorf("child %q not found, available: %s", name, strings.Join(names, ", "))
}

func (c *Client) Children(ctx context.Context) (Children, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return Children{}, err
		}
	}
	return Children{Children: c.children}, nil
}

// UseChild selects a child and holds the selection until release; the portal
// stores it per session, so concurrent tool calls must not interleave.
func (c *Client) UseChild(ctx context.Context, name string) (func(), error) {
	c.opMu.Lock()
	if err := c.selectChild(ctx, name); err != nil {
		c.opMu.Unlock()
		return nil, err
	}
	return c.opMu.Unlock, nil
}

func (c *Client) selectChild(ctx context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return err
		}
	}
	k, err := pickChild(c.children, name)
	if err != nil {
		return err
	}
	c.want = k.ID
	if err := c.applyChild(ctx); !errors.Is(err, errExpired) {
		return err
	}
	// Session expired since the last request; login re-applies c.want.
	return c.login(ctx)
}

// errExpired reports that the portal answered with its login page.
var errExpired = errors.New("session expired")

// applyChild restores the wanted child; each login resets it to the portal default.
func (c *Client) applyChild(ctx context.Context) error {
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
		return fmt.Errorf("switching to child %s failed", c.want)
	}
	c.selected = c.want
	return nil
}
