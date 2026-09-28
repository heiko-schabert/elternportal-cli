package portal

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// pdfText shells out to poppler; no pure-Go extractor matches its layout and
// umlaut handling.
func pdfText(ctx context.Context, pdf []byte) (string, error) {
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", "-", "-")
	cmd.Stdin = bytes.NewReader(pdf)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
