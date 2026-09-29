package portal

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

const childrenHTML = `<html><div class="pupil-selector"><div class="form-group"><select class="form-control" onchange="set_child(this.value)"><option value="1906">Anna Muster (7C)</option><option value="1907" selected>Ben Muster (5A)</option></select></div></div></html>`

func TestParseChildren(t *testing.T) {
	doc, err := parseHTML([]byte(childrenHTML), "")
	if err != nil {
		t.Fatal(err)
	}
	children, sel := parseChildren(doc.Selection)
	want := []Child{{ID: "1906", Name: "Anna Muster", Class: "7C"}, {ID: "1907", Name: "Ben Muster", Class: "5A"}}
	if !reflect.DeepEqual(children, want) || sel != "1907" {
		t.Fatalf("got %+v %q", children, sel)
	}
}

func TestPickChild(t *testing.T) {
	two := []Child{{ID: "1", Name: "Anna Muster"}, {ID: "2", Name: "Ben Muster"}}
	if k, err := pickChild(two, "ben"); err != nil || k.ID != "2" {
		t.Errorf("by first name: %+v %v", k, err)
	}
	if k, err := pickChild(two, "Anna Muster"); err != nil || k.ID != "1" {
		t.Errorf("by full name: %+v %v", k, err)
	}
	if _, err := pickChild(two, ""); err == nil || !strings.Contains(err.Error(), "Anna Muster, Ben Muster") {
		t.Errorf("missing name with two kids: %v", err)
	}
	if _, err := pickChild(two, "Carl"); err == nil || !strings.Contains(err.Error(), "Anna Muster") {
		t.Errorf("unknown name: %v", err)
	}
	if k, err := pickChild(two[:1], ""); err != nil || k.ID != "1" {
		t.Errorf("single kid default: %+v %v", k, err)
	}
	if k, err := pickChild(nil, ""); err != nil || k.ID != "" {
		t.Errorf("no selector: %+v %v", k, err)
	}
}

func useChildPage(t *testing.T, c *Client, name string) string {
	t.Helper()
	release, err := c.UseChild(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	s, err := c.page(context.Background(), "/wer")
	if err != nil {
		t.Fatal(err)
	}
	return text(s)
}

func TestUseChild(t *testing.T) {
	f := newFakePortal(t, nil)
	c := f.client("p")
	if got := useChildPage(t, c, "ben"); got != "1907" {
		t.Fatalf("got %q, want 1907", got)
	}
	if got := useChildPage(t, c, "anna"); got != "1906" {
		t.Fatalf("got %q, want 1906", got)
	}
}

func TestChildReappliedAfterRelogin(t *testing.T) {
	f := newFakePortal(t, nil)
	c := f.client("p")
	useChildPage(t, c, "ben")
	f.expire()
	// Relogin inside the request must restore Ben, not the portal default.
	if got := useChildPage(t, c, "ben"); got != "1907" {
		t.Fatalf("got %q, want 1907", got)
	}
}

func TestSwitchChildAfterExpiry(t *testing.T) {
	f := newFakePortal(t, nil)
	c := f.client("p")
	useChildPage(t, c, "ben")
	f.expire()
	if got := useChildPage(t, c, "anna"); got != "1906" {
		t.Fatalf("got %q, want 1906", got)
	}
}
