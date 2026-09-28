package portal

import (
	"reflect"
	"testing"
)

func TestSchwarzesBrett(t *testing.T) {
	got := parseSchwarzesBrett(fixture(t, "schwarzes_brett.html"))
	want := SchwarzesBrett{Aushaenge: []Aushang{
		{Titel: "Fundsachen", Text: "Bitte bis Freitag abholen.\nDanach Spende."},
		{Titel: "Mensa", Text: "Neuer Speiseplan online."},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSchwarzesBrettArchiv(t *testing.T) {
	got := parseSchwarzesBrett(fixture(t, "schwarzes_brett_echt.html"))
	want := SchwarzesBrett{Aushaenge: []Aushang{
		{Titel: "Feriengrüße", Zeitraum: "30.07.2026 - 11.09.2026", Text: "Sehr geehrte Eltern,\nerholsame Sommerferien!", Archiv: true},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
