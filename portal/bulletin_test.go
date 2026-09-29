package portal

import (
	"reflect"
	"testing"
)

func TestBulletin(t *testing.T) {
	got := parseBulletin(fixture(t, "schwarzes_brett.html"))
	want := Bulletin{Notices: []Notice{
		{Title: "Fundsachen", Text: "Bitte bis Freitag abholen.\nDanach Spende."},
		{Title: "Mensa", Text: "Neuer Speiseplan online."},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestBulletinArchive(t *testing.T) {
	got := parseBulletin(fixture(t, "schwarzes_brett_echt.html"))
	want := Bulletin{Notices: []Notice{
		{Title: "Feriengrüße", Period: "30.07.2026 - 11.09.2026", Text: "Sehr geehrte Eltern,\nerholsame Sommerferien!", Archived: true},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
