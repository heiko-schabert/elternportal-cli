package portal

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func TestRequestLogRedactsCSRF(t *testing.T) {
	buf := captureLog(t)
	f := newFakePortal(t, map[string]string{"/aktuelles/get_file/": "%PDF-1.4 x"})
	if _, _, err := f.client("p").fetch(context.Background(), "/aktuelles/get_file/?repo=1&csrf=geheimtoken"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "geheimtoken") || !strings.Contains(out, "csrf=REDACTED") || !strings.Contains(out, "status=200") {
		t.Fatalf("log:\n%s", out)
	}
}

func TestLoginLogHasNoSecrets(t *testing.T) {
	buf := captureLog(t)
	f := newFakePortal(t, nil)
	f.client("falschXYZ").CheckLogin(context.Background())
	if out := buf.String(); strings.Contains(out, "falschXYZ") || strings.Contains(out, "=tok") {
		t.Fatalf("log leaks secrets:\n%s", out)
	}
}

func TestReloginLogsWarning(t *testing.T) {
	f := newFakePortal(t, map[string]string{"/x": okPage})
	c := f.client("p")
	if _, err := c.page(context.Background(), "/x"); err != nil {
		t.Fatal(err)
	}
	buf := captureLog(t)
	f.expire()
	if _, err := c.page(context.Background(), "/x"); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, "session expired") {
		t.Fatalf("log:\n%s", out)
	}
}
