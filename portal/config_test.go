package portal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnv(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigFile(t *testing.T) {
	p := writeEnv(t, "# comment\nELTERNPORTAL_URL=https://x.eltern-portal.org/\nELTERNPORTAL_USER = a@b.de\nELTERNPORTAL_PASSWORD=\"geheim=1\"\n")
	c, err := LoadConfig(env(nil), p)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{URL: "https://x.eltern-portal.org", User: "a@b.de", Password: "geheim=1"}
	if c != want {
		t.Fatalf("got %+v, want %+v", c, want)
	}
}

func TestLoadConfigEnvWins(t *testing.T) {
	p := writeEnv(t, "ELTERNPORTAL_URL=https://file\nELTERNPORTAL_USER=file\nELTERNPORTAL_PASSWORD=file\n")
	c, err := LoadConfig(env(map[string]string{"ELTERNPORTAL_USER": "env", "ELTERNPORTAL_ALLOW_WRITE": "1"}), p)
	if err != nil {
		t.Fatal(err)
	}
	if c.User != "env" || c.URL != "https://file" || !c.AllowWrite {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	_, err := LoadConfig(env(map[string]string{"ELTERNPORTAL_PASSWORD": "geheim"}), filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("want error")
	}
	for _, k := range []string{"ELTERNPORTAL_URL", "ELTERNPORTAL_USER"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error %q lacks %s", err, k)
		}
	}
	if strings.Contains(err.Error(), "geheim") {
		t.Error("error leaks password")
	}
}

func TestDefaultEnvFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	if got := DefaultEnvFile(); got != "/cfg/elternportal/env" {
		t.Fatalf("got %s", got)
	}
}
