package portal

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestExams(t *testing.T) {
	got := parseEvents(fixture(t, "schulaufgaben.html"))
	want := Events{Class: "6C", Events: []Event{
		{Date: "06.10.2026", Description: "Mathematik Schulaufgabe"},
		{Date: "13.10.2026", Description: "Englisch Stegreifaufgabe"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestExamsEmpty(t *testing.T) {
	b, _ := json.Marshal(parseEvents(fixture(t, "schulaufgaben_leer.html")))
	if !strings.Contains(string(b), `"events":[]`) {
		t.Fatalf("got %s", b)
	}
}

func TestEventsGeneral(t *testing.T) {
	got := parseEvents(fixture(t, "termine_allgemein.html"))
	want := Events{Events: []Event{
		{Date: "01.08.2026 - 14.09.2026", Description: "Sommerferien"},
		{Date: "09.09.2026 - 11.09.2026", Description: "Prüfungstage für Nachprüfungen\nAnsprechpartnerin: Fr. Muster"},
		{Date: "15.09.2026", Time: "08:00 - 11:30", Description: "Jgst. 6-11: Beginn des Unterrichts"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
