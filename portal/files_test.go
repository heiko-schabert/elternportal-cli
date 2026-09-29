package portal

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLetterFileRef(t *testing.T) {
	ls := parseLetters(fixture(t, "elternbriefe.html"))
	ref, ok := ls[0].File()
	if !ok || ref.Name != "letter-134-Wandertag.pdf" {
		t.Fatalf("got %+v %v", ref, ok)
	}
	if _, ok := ls[1].File(); ok {
		t.Fatal("letter without file has a ref")
	}
}

func TestLetterWithFiles(t *testing.T) {
	c := letterPortal(t, "%PDF-1.4 dummy")
	t.Setenv("PATH", "")
	_, files, err := c.Letter(context.Background(), 134, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "letter-134-Wandertag.pdf" || !strings.HasPrefix(string(files[0].Data), "%PDF") {
		t.Fatalf("got %+v", files)
	}
	if _, files, _ := c.Letter(context.Background(), 134, "", false); files != nil {
		t.Fatal("files without withFiles")
	}
}

func TestMessageWithFiles(t *testing.T) {
	t.Setenv("PATH", "")
	_, files, err := messagesPortal(t).client("p").Message(context.Background(), 146807, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "thread-146807-Spenden.pdf" {
		t.Fatalf("got %+v", files)
	}
}

func TestSaveFileSanitizes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "archiv")
	p, err := SaveFile(dir, File{Name: "../../x.pdf", Data: []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(dir, "x.pdf") {
		t.Fatalf("saved to %s", p)
	}
	if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("stat %v, mode %v", err, fi.Mode())
	}
}

func TestSync(t *testing.T) {
	t.Setenv("PATH", "")
	letters, err := os.ReadFile("testdata/elternbriefe.html")
	if err != nil {
		t.Fatal(err)
	}
	f := messagesPortal(t)
	f.pages[lettersPath] = string(letters)
	c := f.client("p")
	dir := t.TempDir()
	res, err := c.Sync(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range res.Saved {
		names = append(names, filepath.Base(p))
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"letter-134-Wandertag.pdf", "thread-146807-Spenden.pdf"}) {
		t.Fatalf("saved %v", names)
	}
	if !slices.Equal(res.SkippedUnread, []int{148884}) || f.hits[teacherMessagesPath+"/62/148884"] != 0 {
		t.Fatalf("unread handling: skipped %v, hits %d", res.SkippedUnread, f.hits[teacherMessagesPath+"/62/148884"])
	}
	downloads := f.hits["/aktuelles/get_file/"]
	res, err = c.Sync(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Saved) != 0 || f.hits["/aktuelles/get_file/"] != downloads {
		t.Fatalf("second run saved %v, downloads %d → %d", res.Saved, downloads, f.hits["/aktuelles/get_file/"])
	}
}
