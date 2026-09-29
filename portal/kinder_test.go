package portal

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

const kinderHTML = `<html><div class="pupil-selector"><div class="form-group"><select class="form-control" onchange="set_child(this.value)"><option value="1906">Anna Muster (7C)</option><option value="1907" selected>Ben Muster (5A)</option></select></div></div></html>`

func TestParseKinder(t *testing.T) {
	doc, err := parseHTML([]byte(kinderHTML), "")
	if err != nil {
		t.Fatal(err)
	}
	kinder, sel := parseKinder(doc.Selection)
	want := []Kind{{ID: "1906", Name: "Anna Muster", Klasse: "7C"}, {ID: "1907", Name: "Ben Muster", Klasse: "5A"}}
	if !reflect.DeepEqual(kinder, want) || sel != "1907" {
		t.Fatalf("got %+v %q", kinder, sel)
	}
}

func TestPickKind(t *testing.T) {
	two := []Kind{{ID: "1", Name: "Anna Muster"}, {ID: "2", Name: "Ben Muster"}}
	if k, err := pickKind(two, "ben"); err != nil || k.ID != "2" {
		t.Errorf("by first name: %+v %v", k, err)
	}
	if k, err := pickKind(two, "Anna Muster"); err != nil || k.ID != "1" {
		t.Errorf("by full name: %+v %v", k, err)
	}
	if _, err := pickKind(two, ""); err == nil || !strings.Contains(err.Error(), "Anna Muster, Ben Muster") {
		t.Errorf("missing name with two kids: %v", err)
	}
	if _, err := pickKind(two, "Carl"); err == nil || !strings.Contains(err.Error(), "Anna Muster") {
		t.Errorf("unknown name: %v", err)
	}
	if k, err := pickKind(two[:1], ""); err != nil || k.ID != "1" {
		t.Errorf("single kid default: %+v %v", k, err)
	}
	if k, err := pickKind(nil, ""); err != nil || k.ID != "" {
		t.Errorf("no selector: %+v %v", k, err)
	}
}

func useKindPage(t *testing.T, c *Client, name string) string {
	t.Helper()
	release, err := c.UseKind(context.Background(), name)
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

func TestUseKind(t *testing.T) {
	f := newFakePortal(t, nil)
	c := f.client("p")
	if got := useKindPage(t, c, "ben"); got != "1907" {
		t.Fatalf("got %q, want 1907", got)
	}
	if got := useKindPage(t, c, "anna"); got != "1906" {
		t.Fatalf("got %q, want 1906", got)
	}
}

func TestKindReappliedAfterRelogin(t *testing.T) {
	f := newFakePortal(t, nil)
	c := f.client("p")
	useKindPage(t, c, "ben")
	f.expire()
	// Relogin inside the request must restore Ben, not the portal default.
	if got := useKindPage(t, c, "ben"); got != "1907" {
		t.Fatalf("got %q, want 1907", got)
	}
}

func TestSwitchKindAfterExpiry(t *testing.T) {
	f := newFakePortal(t, nil)
	c := f.client("p")
	useKindPage(t, c, "ben")
	f.expire()
	if got := useKindPage(t, c, "anna"); got != "1906" {
		t.Fatalf("got %q, want 1906", got)
	}
}
