package main

import (
	"bytes"
	"strings"
	"testing"
)

func renderString(t *testing.T, js string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := render(&buf, []byte(js)); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestRenderTable(t *testing.T) {
	got := renderString(t, `{"letters":[{"number":50,"title":"Wahl","confirmed":true,"file_name":""},{"number":49,"title":"Konzept","confirmed":false,"file_name":"a.pdf"}]}`)
	want := "NUMBER  TITLE    CONFIRMED  FILE NAME\n" +
		"50      Wahl     yes\n" +
		"49      Konzept  no         a.pdf\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderObjectWithText(t *testing.T) {
	got := renderString(t, `{"number":50,"title":"Wahl","classes":"","content":"Sehr geehrte Eltern,\nText."}`)
	want := "Number:  50\nTitle:   Wahl\n\nSehr geehrte Eltern,\nText.\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderNestedBlocks(t *testing.T) {
	got := renderString(t, `{"subject":"Frage","posts":[{"from":"Lehrer","text":"Hallo\nWelt","attachments":[{"name":"a.pdf","content":"PDF\ntext"}]}]}`)
	for _, want := range []string{"Subject:  Frage", "Posts:", "From:  Lehrer", "Hallo\n", "Attachments:", "Name:  a.pdf", "PDF"} {
		if !strings.Contains(got, want) {
			t.Errorf("lacks %q:\n%s", want, got)
		}
	}
}

func TestRenderScalarsAndEmpty(t *testing.T) {
	got := renderString(t, `{"saved":["/a/x.pdf","/a/y.pdf"],"skipped_unread":[]}`)
	want := "Saved:\n  /a/x.pdf\n  /a/y.pdf\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
	if got := renderString(t, `{"letters":[]}`); got != "(none)\n" {
		t.Fatalf("empty list: %q", got)
	}
}
