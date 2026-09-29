package portal

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

const insPath = "/includes/meldungen_dir/kommunikation_fachlehrer_db_ins.php"

func sendPortal(t *testing.T) *fakePortal {
	t.Helper()
	read := func(n string) string {
		b, err := os.ReadFile("testdata/" + n)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	f := newFakePortal(t, map[string]string{
		teacherMessagesPath:                read("nachrichten.html"),
		teacherMessagesPath + "/29/146807": read("thread.html"),
		teacherMessagesPath + "/29":        read("neu_fachlehrer.html"),
		"/meldungen/kommunikation/neu":     read("neu_klassenleitung.html"),
	})
	// Echo sent text into the thread / subject into the list, like the portal would.
	f.onPost = func(path string, v map[string][]string) {
		switch {
		case path == insPath && len(v["kob_id"]) > 0:
			f.pages[teacherMessagesPath+"/29/146807"] = strings.Replace(f.pages[teacherMessagesPath+"/29/146807"],
				`<div class="row"> <div class="four wide right aligned column"><strong><span class="ui green text">`,
				`<div class="row"> <div class="four wide right aligned column"><strong><span class="ui text">Ich</span>:</strong> (29.09.2026, 10:00)</div> <div class="twelve wide column"><div class="ui segment"><div>`+strings.ReplaceAll(v["nachricht_kom_fach"][0], "\n", "<br />\n")+`</div></div></div> </div><div class="row"> <div class="four wide right aligned column"><strong><span class="ui green text">`, 1)
		case path == insPath:
			f.pages[teacherMessagesPath] = strings.Replace(f.pages[teacherMessagesPath], "Taschenrechner Bestellung", " "+v["new_betreff"][0]+" ", 1)
		}
	}
	return f
}

func TestReply(t *testing.T) {
	f := sendPortal(t)
	th, err := f.client("p").Reply(context.Background(), 146807, "Danke!")
	if err != nil {
		t.Fatal(err)
	}
	p := f.posts[insPath]
	if p.Get("csrf") != "c0ffee" || p.Get("kob_id") != "146807" || p.Get("teacher_id") != "29" || p.Get("nachricht_kom_fach") != "Danke!" {
		t.Fatalf("posted %v", p)
	}
	if last := th.Posts[len(th.Posts)-1]; last.Text != "Danke!" {
		t.Fatalf("last = %+v", last)
	}
}

func TestReplyEmpty(t *testing.T) {
	f := sendPortal(t)
	if _, err := f.client("p").Reply(context.Background(), 146807, "  "); err == nil {
		t.Fatal("want error for empty text")
	}
	if len(f.posts) != 0 {
		t.Fatal("posted empty message")
	}
}

func TestNewMessage(t *testing.T) {
	f := sendPortal(t)
	n, err := f.client("p").NewMessage(context.Background(), 29, "Frage Hausaufgaben", "Hallo")
	if err != nil {
		t.Fatal(err)
	}
	p := f.posts[insPath]
	if p.Get("teacher_id") != "29" || p.Get("new_betreff") != "Frage Hausaufgaben" || p.Get("nachricht_kom_fach") != "Hallo" || p.Get("csrf") != "c0ffee" {
		t.Fatalf("posted %v", p)
	}
	if n.Subject != "Frage Hausaufgaben" {
		t.Fatalf("got %+v", n)
	}
}

func TestNewMessageSubjectTooLong(t *testing.T) {
	f := sendPortal(t)
	if _, err := f.client("p").NewMessage(context.Background(), 29, strings.Repeat("x", 129), "Hallo"); err == nil {
		t.Fatal("want error for subject > 128")
	}
}

func TestContactClassTeacher(t *testing.T) {
	f := sendPortal(t)
	if err := f.client("p").ContactClassTeacher(context.Background(), "telefontermin", "Bitte Rückruf"); err != nil {
		t.Fatal(err)
	}
	p := f.posts["/includes/meldungen_dir/kommunikation_db_ins.php"]
	if p.Get("betreff") != "Telefontermin" || p.Get("kommentar") != "Bitte Rückruf" || p.Get("csrf") != "c0ffee" {
		t.Fatalf("posted %v", p)
	}
}

func TestContactClassTeacherInvalid(t *testing.T) {
	f := sendPortal(t)
	err := f.client("p").ContactClassTeacher(context.Background(), "Kaffee", "x")
	if err == nil || !strings.Contains(err.Error(), "Beratungsgespräch") {
		t.Fatalf("err = %v, want valid options listed", err)
	}
	if len(f.posts) != 0 {
		t.Fatal("posted invalid request")
	}
}

func TestReplyMultiline(t *testing.T) {
	f := sendPortal(t)
	if _, err := f.client("p").Reply(context.Background(), 146807, "Hallo,\n\nDanke!"); err != nil {
		t.Fatal(err)
	}
}

func TestNewMessageSubjectSpaces(t *testing.T) {
	f := sendPortal(t)
	n, err := f.client("p").NewMessage(context.Background(), 29, "Frage  Hausaufgaben", "Hallo")
	if err != nil {
		t.Fatal(err)
	}
	if n.Subject != "Frage Hausaufgaben" {
		t.Fatalf("got %+v", n)
	}
}

func TestSendTimeout(t *testing.T) {
	f := sendPortal(t)
	c := f.client("p")
	if _, err := c.Messages(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	c.http.Timeout = 50 * time.Millisecond
	f.onPost = func(string, map[string][]string) { time.Sleep(200 * time.Millisecond) }
	err := c.ContactClassTeacher(context.Background(), "Telefontermin", "x")
	if err == nil || !strings.Contains(err.Error(), "check the portal") {
		t.Fatalf("err = %v, want unknown-status warning", err)
	}
}
