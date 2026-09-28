package portal

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSchulaufgaben(t *testing.T) {
	got := parseTermine(fixture(t, "schulaufgaben.html"))
	want := Termine{Klasse: "6C", Termine: []Termin{
		{Datum: "06.10.2026", Beschreibung: "Mathematik Schulaufgabe"},
		{Datum: "13.10.2026", Beschreibung: "Englisch Stegreifaufgabe"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSchulaufgabenLeer(t *testing.T) {
	b, _ := json.Marshal(parseTermine(fixture(t, "schulaufgaben_leer.html")))
	if !strings.Contains(string(b), `"termine":[]`) {
		t.Fatalf("got %s", b)
	}
}

func TestTermineAllgemein(t *testing.T) {
	got := parseTermine(fixture(t, "termine_allgemein.html"))
	want := Termine{Termine: []Termin{
		{Datum: "01.08.2026 - 14.09.2026", Beschreibung: "Sommerferien"},
		{Datum: "09.09.2026 - 11.09.2026", Beschreibung: "Prüfungstage für Nachprüfungen\nAnsprechpartnerin: Fr. Muster"},
		{Datum: "15.09.2026", Zeit: "08:00 - 11:30", Beschreibung: "Jgst. 6-11: Beginn des Unterrichts"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
