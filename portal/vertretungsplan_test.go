package portal

import (
	"reflect"
	"testing"
)

func TestVertretungsplan(t *testing.T) {
	got := parseVertretungsplan(fixture(t, "vertretungsplan.html"))
	want := Vertretungsplan{
		Stand: "14.03.2026 08:12:05",
		Tage: []Tag{
			{Datum: "Fr., 14.03.2026", KW: "KW 11", Vertretungen: []Vertretung{
				{Stunde: "3.", Betrifft: "Kp", Vertretung: "Sm", FachAlt: "M", Fach: "NuT_1", Raum: "A204", Info: "Vertretung"},
				{Stunde: "5.", Betrifft: "Ro", Vertretung: "---", Fach: "E", Raum: "A105", Info: "Entfall"},
			}},
			{Datum: "Mo., 17.03.2026", KW: "KW 12", Vertretungen: []Vertretung{}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
