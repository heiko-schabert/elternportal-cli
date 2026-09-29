package portal

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func docFixture(t *testing.T, name string) *goquery.Document {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	d, err := parseHTML(b, "text/html; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestParseNachrichten(t *testing.T) {
	got, err := parseNachrichten(docFixture(t, "nachrichten.html"))
	if err != nil {
		t.Fatal(err)
	}
	want := Nachrichten{Seite: 1, Seiten: 2, Nachrichten: []Nachricht{
		{LehrerID: 62, ThreadID: 148884, Lehrkraft: "Max Lehrer", Betreff: "Taschenrechner Bestellung", Datum: "28.09.2026, 12:22", VonLehrkraft: true, Anhang: true, Ungelesen: true},
		{LehrerID: 29, ThreadID: 146807, Lehrkraft: "Erika Lehrerin", Betreff: "fehlender Anhang", Datum: "23.07.2026, 12:36"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseNachrichtenLayout(t *testing.T) {
	d, _ := parseHTML([]byte("<html>anders</html>"), "")
	if _, err := parseNachrichten(d); err == nil {
		t.Fatal("want error when table missing")
	}
}

func TestParseThread(t *testing.T) {
	got, err := parseThread(docFixture(t, "thread.html"))
	if err != nil {
		t.Fatal(err)
	}
	want := Thread{Betreff: "fehlender Anhang", Fuer: "Anna Muster", Beitraege: []Beitrag{
		{Von: "Erika Lehrerin", Datum: "23.07.2026, 12:36", Text: "Bitte um Entschuldigung.\nAnhang anbei.",
			Anhaenge: []Anhang{{Name: "Spenden.pdf", href: "aktuelles/get_file/?repo=63501&csrf=c0ffee"}}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseLehrkraefte(t *testing.T) {
	got, err := parseLehrkraefte(docFixture(t, "lehrkraefte.html"))
	if err != nil {
		t.Fatal(err)
	}
	want := Lehrkraefte{Lehrkraefte: []Lehrkraft{{ID: 239, Name: "Lehrer, Max"}, {ID: 3, Name: "Lehrerin, Erika", Funktion: "StDin, 3. Klassenleitung"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func nachrichtenPortal(t *testing.T) *fakePortal {
	t.Helper()
	read := func(n string) string {
		b, err := os.ReadFile("testdata/" + n)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return newFakePortal(t, map[string]string{
		fachlehrerPath:                read("nachrichten.html"),
		fachlehrerPath + "/29/146807": read("thread.html"),
		fachlehrerPath + "/62/148884": read("thread.html"),
		"/aktuelles/get_file/":        "%PDF-1.4 dummy",
	})
}

func TestNachrichtUngelesenGuard(t *testing.T) {
	f := nachrichtenPortal(t)
	_, err := f.client("p").Nachricht(context.Background(), 62, 148884, false)
	if err == nil || !strings.Contains(err.Error(), "ungelesen") {
		t.Fatalf("err = %v, want unread guard", err)
	}
	if f.hits[fachlehrerPath+"/62/148884"] != 0 {
		t.Fatal("unread thread was opened")
	}
}

func TestNachrichtOeffnen(t *testing.T) {
	t.Setenv("PATH", "")
	f := nachrichtenPortal(t)
	th, err := f.client("p").Nachricht(context.Background(), 62, 148884, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := th.Beitraege[0].Anhaenge[0].Inhalt; !strings.Contains(got, "pdftotext nicht installiert") {
		t.Fatalf("attachment = %q", got)
	}
}

func TestNachrichtBildAnhang(t *testing.T) {
	f := nachrichtenPortal(t)
	f.pages["/aktuelles/get_file/"] = "\x89PNG\r\n\x1a\n bild"
	th, err := f.client("p").Nachricht(context.Background(), 29, 146807, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := th.Beitraege[0].Anhaenge[0].Inhalt; !strings.Contains(got, "kein Text") {
		t.Fatalf("attachment = %q", got)
	}
}

func TestNachrichtUnbekannt(t *testing.T) {
	f := nachrichtenPortal(t)
	_, err := f.client("p").Nachricht(context.Background(), 1, 1, false)
	if err == nil || !strings.Contains(err.Error(), "nicht in der Liste") {
		t.Fatalf("err = %v, want not-found guard", err)
	}
	if f.hits[fachlehrerPath+"/1/1"] != 0 {
		t.Fatal("unknown thread was opened")
	}
}
