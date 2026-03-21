package output

import (
	"bytes"
	"testing"
)

func TestStatusLineFormatsOutput(t *testing.T) {
	var buf bytes.Buffer

	if err := StatusLine(&buf, StatusOK, "config file found"); err != nil {
		t.Fatalf("StatusLine returned error: %v", err)
	}

	if buf.String() != "✅ config file found\n" {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}
