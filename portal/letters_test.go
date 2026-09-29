package portal

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestParseLetters(t *testing.T) {
	got := parseLetters(fixture(t, "elternbriefe.html"))
	want := []Letter{
		{Number: 134, Title: "Wandertag", Date: "20.09.2026, 17:30", Classes: "6C, 6D", HasFile: true, FileName: "Wandertag.pdf", downloadURL: "aktuelles/get_file/?repo=134&csrf=c0ffee", id: "1300", inline: "Anbei die Infos zum Wandertag."},
		{Number: 133, Title: "Elternabend", Date: "15.09.2026, 08:00", Classes: "6C", Confirmed: true, id: "1299", inline: "Sehr geehrte Eltern,\nder Elternabend findet am 1.10. statt.\nMit freundlichen Grüßen\ni.A."},
		{Number: 120, Title: "Elternabend Nachtrag", Date: "01.09.2026, 08:00", Classes: "6C", Confirmed: true, id: "1200", inline: "Raum 101."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestFindLetter(t *testing.T) {
	bs := parseLetters(fixture(t, "elternbriefe.html"))
	if b, err := findLetter(bs, 120, ""); err != nil || b.Number != 120 {
		t.Errorf("by number: %v, %v", b.Number, err)
	}
	// Title match prefers the newest letter.
	if b, err := findLetter(bs, 0, "elternABEND"); err != nil || b.Number != 133 {
		t.Errorf("by title: %v, %v", b.Number, err)
	}
	if _, err := findLetter(bs, 999, ""); err == nil || !strings.Contains(err.Error(), "#134") {
		t.Errorf("not found should list available: %v", err)
	}
	if _, err := findLetter(bs, 0, ""); err == nil {
		t.Error("want error without number and title")
	}
}

func letterPortal(t *testing.T, download string) *Client {
	b, err := os.ReadFile("testdata/elternbriefe.html")
	if err != nil {
		t.Fatal(err)
	}
	return newFakePortal(t, map[string]string{
		"/aktuelles/elternbriefe": string(b),
		"/aktuelles/get_file/":    download,
	}).client("p")
}

func TestLetterInline(t *testing.T) {
	got, _, err := letterPortal(t, "").Letter(context.Background(), 133, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "Sehr geehrte Eltern,\nder Elternabend findet am 1.10. statt.\nMit freundlichen Grüßen\ni.A." {
		t.Fatalf("got %q", got.Content)
	}
}

func TestLetterDownloadHTML(t *testing.T) {
	_, _, err := letterPortal(t, "<html>Fehler</html>").Letter(context.Background(), 134, "", false)
	if err == nil || !strings.Contains(err.Error(), "HTML") {
		t.Fatalf("err = %v, want HTML download error", err)
	}
}

func TestLetterWithoutPdftotext(t *testing.T) {
	t.Setenv("PATH", "")
	got, _, err := letterPortal(t, "%PDF-1.4 dummy").Letter(context.Background(), 134, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "pdftotext not installed") {
		t.Fatalf("got %q", got.Content)
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

func TestConfirmLetter(t *testing.T) {
	b, err := os.ReadFile("testdata/elternbriefe.html")
	if err != nil {
		t.Fatal(err)
	}
	f := newFakePortal(t, map[string]string{"/aktuelles/elternbriefe": string(b), "/api/elternbrief_bestaetigen.php": "ok"})
	// Fake flips the letter's status once the confirm endpoint is hit.
	f.onHit = func(path string) {
		if path == "/api/elternbrief_bestaetigen.php" {
			f.pages["/aktuelles/elternbriefe"] = strings.Replace(string(b), "noch nicht bestätigt", "Empfang bestätigt.", 1)
		}
	}
	got, err := f.client("p").ConfirmLetter(context.Background(), 134)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Confirmed || f.lastQuery["/api/elternbrief_bestaetigen.php"] != "eb=1300" {
		t.Fatalf("got %+v, query %q", got, f.lastQuery["/api/elternbrief_bestaetigen.php"])
	}
}

func TestConfirmLetterAlreadyConfirmed(t *testing.T) {
	b, _ := os.ReadFile("testdata/elternbriefe.html")
	f := newFakePortal(t, map[string]string{"/aktuelles/elternbriefe": string(b)})
	got, err := f.client("p").ConfirmLetter(context.Background(), 133)
	if err != nil || !got.Confirmed {
		t.Fatalf("got %+v %v", got, err)
	}
	if f.hits["/api/elternbrief_bestaetigen.php"] != 0 {
		t.Fatal("confirmed an already confirmed letter")
	}
}
