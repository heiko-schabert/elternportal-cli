package portal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// File is an original document as the portal serves it. Name is the on-disk
// name; its letter-/thread- prefix keeps names unique across the archive.
type File struct {
	Name        string
	ContentType string
	Data        []byte
}

// FileRef names a file before it is downloaded, so Sync can skip existing ones.
type FileRef struct {
	Name string
	href string
}

type SyncResult struct {
	Saved         []string `json:"saved"`
	SkippedUnread []int    `json:"skipped_unread"`
}

// safeName keeps portal-supplied names from becoming paths.
func safeName(name string) string {
	name = filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), `\`, "/"))
	if name == "." || name == ".." || name == "/" {
		return "file"
	}
	return name
}

func (c *Client) Download(ctx context.Context, ref FileRef) (File, error) {
	path, err := c.resolve(ref.href)
	if err != nil {
		return File{}, err
	}
	raw, ct, err := c.fetch(ctx, path)
	if err != nil {
		return File{}, err
	}
	sniffed := http.DetectContentType(raw)
	if strings.Contains(ct, "html") || strings.Contains(sniffed, "html") {
		return File{}, fmt.Errorf("%s: download returned HTML instead of a file", path)
	}
	if ct == "" {
		ct = sniffed
	}
	return File{Name: ref.Name, ContentType: ct, Data: raw}, nil
}

// fileText extracts a PDF's text; other types only get a note.
func fileText(ctx context.Context, f File) (string, error) {
	if !bytes.HasPrefix(f.Data, []byte("%PDF-")) {
		return fmt.Sprintf("(attachment %s, no text)", http.DetectContentType(f.Data)), nil
	}
	txt, err := pdfText(ctx, f.Data)
	if errors.Is(err, exec.ErrNotFound) {
		return "(pdftotext not installed, PDF text unavailable)", nil
	}
	return txt, err
}

// SaveFile writes f into dir, readable only by the owner: these are
// children's school records.
func SaveFile(dir string, f File) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, safeName(f.Name))
	return p, os.WriteFile(p, f.Data, 0o600)
}

// Sync saves letter and message files missing in dir. Unread threads are
// reported, not opened, because opening marks them read. Every read thread is
// checked: the list's attachment flag only covers the last post.
func (c *Client) Sync(ctx context.Context, dir string) (SyncResult, error) {
	res := SyncResult{Saved: []string{}, SkippedUnread: []int{}}
	letters, err := c.Letters(ctx)
	if err != nil {
		return res, err
	}
	var refs []FileRef
	for _, l := range letters.Letters {
		if ref, ok := l.File(); ok {
			refs = append(refs, ref)
		}
	}
	seen := map[int]bool{}
	for page, pages := 1, 1; page <= pages; page++ {
		n, err := c.Messages(ctx, page)
		if err != nil {
			return res, err
		}
		pages = n.Pages
		for _, m := range n.Messages {
			if seen[m.ThreadID] {
				continue
			}
			seen[m.ThreadID] = true
			if m.Unread {
				slog.WarnContext(ctx, "skipping unread thread", "thread", m.ThreadID)
				res.SkippedUnread = append(res.SkippedUnread, m.ThreadID)
				continue
			}
			r, err := c.ThreadFiles(ctx, m, false)
			if err != nil {
				return res, err
			}
			refs = append(refs, r...)
		}
	}
	for _, ref := range refs {
		if _, err := os.Stat(filepath.Join(dir, safeName(ref.Name))); err == nil {
			continue
		}
		f, err := c.Download(ctx, ref)
		if err != nil {
			return res, fmt.Errorf("%s: %w", ref.Name, err)
		}
		p, err := SaveFile(dir, f)
		if err != nil {
			return res, err
		}
		slog.InfoContext(ctx, "saved file", "path", p)
		res.Saved = append(res.Saved, p)
	}
	slog.InfoContext(ctx, "sync done", "saved", len(res.Saved), "skipped_unread", len(res.SkippedUnread))
	return res, nil
}
