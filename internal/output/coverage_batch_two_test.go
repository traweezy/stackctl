package output

import (
	"bytes"
	"os"
	"testing"
)

func TestMarkdownCoverageBatchTwo(t *testing.T) {
	t.Run("default markdown renderer can be constructed", func(t *testing.T) {
		renderer, err := newMarkdownRenderer(bytes.NewBuffer(nil))
		if err != nil {
			t.Fatalf("newMarkdownRenderer returned error: %v", err)
		}
		if renderer == nil {
			t.Fatal("expected a markdown renderer instance")
		}
	})

	t.Run("writer file descriptor accepts real files", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "markdown-fd-*.txt")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		defer func() { _ = file.Close() }()

		fd, ok := writerFileDescriptor(file)
		if !ok || fd < 0 {
			t.Fatalf("expected a valid file descriptor, got fd=%d ok=%v", fd, ok)
		}
	})

	t.Run("terminal writer check handles real files", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "markdown-term-*.txt")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		defer func() { _ = file.Close() }()

		if isTerminalWriter(file) {
			t.Fatal("temp files should not be reported as terminals")
		}
	})
}
