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

func TestParseMessages(t *testing.T) {
	got, err := parseMessages(docFixture(t, "nachrichten.html"))
	if err != nil {
		t.Fatal(err)
	}
	want := Messages{Page: 1, Pages: 2, Messages: []Message{
		{TeacherID: 62, ThreadID: 148884, Teacher: "Max Lehrer", Subject: "Taschenrechner Bestellung", Date: "28.09.2026, 12:22", FromTeacher: true, Attachment: true, Unread: true},
		{TeacherID: 29, ThreadID: 146807, Teacher: "Erika Lehrerin", Subject: "fehlender Anhang", Date: "23.07.2026, 12:36"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseMessagesLayout(t *testing.T) {
	d, _ := parseHTML([]byte("<html>anders</html>"), "")
	if _, err := parseMessages(d); err == nil {
		t.Fatal("want error when table missing")
	}
}

func TestParseThread(t *testing.T) {
	got, err := parseThread(docFixture(t, "thread.html"))
	if err != nil {
		t.Fatal(err)
	}
	want := Thread{Subject: "fehlender Anhang", Child: "Anna Muster", Posts: []Post{
		{From: "Erika Lehrerin", Date: "23.07.2026, 12:36", Text: "Bitte um Entschuldigung.\nAnhang anbei.",
			Attachments: []Attachment{{Name: "Spenden.pdf", href: "aktuelles/get_file/?repo=63501&csrf=c0ffee"}}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseTeachers(t *testing.T) {
	got, err := parseTeachers(docFixture(t, "lehrkraefte.html"))
	if err != nil {
		t.Fatal(err)
	}
	want := Teachers{Teachers: []Teacher{{ID: 239, Name: "Lehrer, Max"}, {ID: 3, Name: "Lehrerin, Erika", Role: "StDin, 3. Klassenleitung"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func messagesPortal(t *testing.T) *fakePortal {
	t.Helper()
	read := func(n string) string {
		b, err := os.ReadFile("testdata/" + n)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return newFakePortal(t, map[string]string{
		teacherMessagesPath:                read("nachrichten.html"),
		teacherMessagesPath + "/29/146807": read("thread.html"),
		teacherMessagesPath + "/62/148884": read("thread.html"),
		"/aktuelles/get_file/":             "%PDF-1.4 dummy",
	})
}

func TestMessageUnreadGuard(t *testing.T) {
	f := messagesPortal(t)
	_, _, err := f.client("p").Message(context.Background(), 148884, false, false)
	if err == nil || !strings.Contains(err.Error(), "unread") {
		t.Fatalf("err = %v, want unread guard", err)
	}
	if f.hits[teacherMessagesPath+"/62/148884"] != 0 {
		t.Fatal("unread thread was opened")
	}
}

func TestMessageOpen(t *testing.T) {
	t.Setenv("PATH", "")
	f := messagesPortal(t)
	th, _, err := f.client("p").Message(context.Background(), 148884, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := th.Posts[0].Attachments[0].Content; !strings.Contains(got, "pdftotext not installed") {
		t.Fatalf("attachment = %q", got)
	}
}

func TestMessageImageAttachment(t *testing.T) {
	f := messagesPortal(t)
	f.pages["/aktuelles/get_file/"] = "\x89PNG\r\n\x1a\n bild"
	th, _, err := f.client("p").Message(context.Background(), 146807, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := th.Posts[0].Attachments[0].Content; !strings.Contains(got, "no text") {
		t.Fatalf("attachment = %q", got)
	}
}

func TestMessageUnknown(t *testing.T) {
	f := messagesPortal(t)
	_, _, err := f.client("p").Message(context.Background(), 1, false, false)
	if err == nil || !strings.Contains(err.Error(), "not in the list") {
		t.Fatalf("err = %v, want not-found guard", err)
	}
	for p, n := range f.hits {
		if strings.HasPrefix(p, teacherMessagesPath+"/") && n > 0 {
			t.Fatalf("opened %s", p)
		}
	}
}

func TestMessageTeacherFromList(t *testing.T) {
	t.Setenv("PATH", "")
	f := messagesPortal(t)
	if _, _, err := f.client("p").Message(context.Background(), 148884, true, false); err != nil {
		t.Fatal(err)
	}
	if f.hits[teacherMessagesPath+"/62/148884"] != 1 {
		t.Fatalf("hits = %v, want thread opened under teacher 62", f.hits)
	}
}
