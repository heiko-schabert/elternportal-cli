package portal

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestParseElternbriefe(t *testing.T) {
	got := parseElternbriefe(fixture(t, "elternbriefe.html"))
	want := []Elternbrief{
		{Nummer: 134, Titel: "Wandertag", Datum: "20.09.2026, 17:30", Klassen: "6C, 6D", HatDatei: true, downloadURL: "aktuelles/get_file/?repo=134&csrf=c0ffee", inline: "Anbei die Infos zum Wandertag."},
		{Nummer: 133, Titel: "Elternabend", Datum: "15.09.2026, 08:00", Klassen: "6C", Bestaetigt: true, inline: "Sehr geehrte Eltern,\nder Elternabend findet am 1.10. statt.\nMit freundlichen Grüßen\ni.A."},
		{Nummer: 120, Titel: "Elternabend Nachtrag", Datum: "01.09.2026, 08:00", Klassen: "6C", Bestaetigt: true, inline: "Raum 101."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestFindBrief(t *testing.T) {
	bs := parseElternbriefe(fixture(t, "elternbriefe.html"))
	if b, err := findBrief(bs, 120, ""); err != nil || b.Nummer != 120 {
		t.Errorf("by number: %v, %v", b.Nummer, err)
	}
	// Title match prefers the newest letter.
	if b, err := findBrief(bs, 0, "elternABEND"); err != nil || b.Nummer != 133 {
		t.Errorf("by title: %v, %v", b.Nummer, err)
	}
	if _, err := findBrief(bs, 999, ""); err == nil || !strings.Contains(err.Error(), "#134") {
		t.Errorf("not found should list available: %v", err)
	}
	if _, err := findBrief(bs, 0, ""); err == nil {
		t.Error("want error without nummer and titel")
	}
}

func briefPortal(t *testing.T, download string) *Client {
	b, err := os.ReadFile("testdata/elternbriefe.html")
	if err != nil {
		t.Fatal(err)
	}
	return newFakePortal(t, map[string]string{
		"/aktuelles/elternbriefe": string(b),
		"/aktuelles/get_file/":    download,
	}).client("p")
}

func TestElternbriefInline(t *testing.T) {
	got, err := briefPortal(t, "").Elternbrief(context.Background(), 133, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Inhalt != "Sehr geehrte Eltern,\nder Elternabend findet am 1.10. statt.\nMit freundlichen Grüßen\ni.A." {
		t.Fatalf("got %q", got.Inhalt)
	}
}

func TestElternbriefDownloadHTML(t *testing.T) {
	_, err := briefPortal(t, "<html>Fehler</html>").Elternbrief(context.Background(), 134, "")
	if err == nil || !strings.Contains(err.Error(), "HTML") {
		t.Fatalf("err = %v, want HTML download error", err)
	}
}

func TestElternbriefOhnePdftotext(t *testing.T) {
	t.Setenv("PATH", "")
	got, err := briefPortal(t, "%PDF-1.4 dummy").Elternbrief(context.Background(), 134, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Inhalt, "pdftotext nicht installiert") {
		t.Fatalf("got %q", got.Inhalt)
	}
}

// Minimal one-page PDF; pdftotext rebuilds the missing xref table.
const miniPDF = `%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 300 100]/Contents 4 0 R/Resources<</Font<</F1 5 0 R>>>>>>endobj
4 0 obj<</Length 39>>stream
BT /F1 18 Tf 20 40 Td (Wandertag) Tj ET
endstream endobj
5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj
trailer<</Root 1 0 R>>
%%EOF
`

func TestPDFText(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}
	got, err := pdfText(context.Background(), []byte(miniPDF))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Wandertag") {
		t.Fatalf("got %q", got)
	}
}
