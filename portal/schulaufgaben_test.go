package portal

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSchulaufgaben(t *testing.T) {
	got := parseSchulaufgaben(fixture(t, "schulaufgaben.html"))
	want := Schulaufgaben{Klasse: "6C", Termine: []Termin{
		{Datum: "06.10.2026", Beschreibung: "Mathematik Schulaufgabe"},
		{Datum: "13.10.2026", Beschreibung: "Englisch Stegreifaufgabe"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSchulaufgabenLeer(t *testing.T) {
	b, _ := json.Marshal(parseSchulaufgaben(fixture(t, "schulaufgaben_leer.html")))
	if !strings.Contains(string(b), `"termine":[]`) {
		t.Fatalf("got %s", b)
	}
}
