package portal

import (
	"reflect"
	"testing"
)

func TestSubstitutionPlan(t *testing.T) {
	got := parseSubstitutionPlan(fixture(t, "vertretungsplan.html"))
	want := SubstitutionPlan{
		Updated: "14.03.2026 08:12:05",
		Days: []Day{
			{Date: "Fr., 14.03.2026", Week: "KW 11", Substitutions: []Substitution{
				{Lesson: "3.", Teacher: "Kp", Substitution: "Sm", CancelledSubject: "M", Subject: "NuT_1", Room: "A204", Info: "Vertretung"},
				{Lesson: "5.", Teacher: "Ro", Substitution: "---", Subject: "E", Room: "A105", Info: "Entfall"},
			}},
			{Date: "Mo., 17.03.2026", Week: "KW 12", Substitutions: []Substitution{}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
