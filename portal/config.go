package portal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Config holds portal credentials.
type Config struct {
	URL, User, Password string
	AllowWrite          bool
}

// DefaultEnvFile is shared with the Python elternportal-mcp so either server
// works from the same credentials.
func DefaultEnvFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mcp-server-config", "elternportal_mcp", ".env")
}

// LoadConfig prefers env vars over the file so an MCP client config can override it.
func LoadConfig(getenv func(string) string, envFile string) (Config, error) {
	file := map[string]string{}
	b, err := os.ReadFile(envFile)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			file[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	get := func(k string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return file[k]
	}
	c := Config{
		URL:        strings.TrimRight(get("ELTERNPORTAL_URL"), "/"),
		User:       get("ELTERNPORTAL_USER"),
		Password:   get("ELTERNPORTAL_PASSWORD"),
		AllowWrite: get("ELTERNPORTAL_ALLOW_WRITE") == "1",
	}
	var missing []string
	for k, v := range map[string]string{"ELTERNPORTAL_URL": c.URL, "ELTERNPORTAL_USER": c.User, "ELTERNPORTAL_PASSWORD": c.Password} {
		if v == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return Config{}, fmt.Errorf("Config fehlt: %s (Env-Var oder %s)", strings.Join(missing, ", "), envFile)
	}
	return c, nil
}
