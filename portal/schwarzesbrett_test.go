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
