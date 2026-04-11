package output

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	glamour "charm.land/glamour/v2"
)

type oversizedFDWriter struct{}

func (oversizedFDWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (oversizedFDWriter) Fd() uintptr {
	return ^uintptr(0)
}

func TestIsTerminalWriterRejectsOutOfRangeFD(t *testing.T) {
	if isTerminalWriter(oversizedFDWriter{}) {
		t.Fatal("expected oversized file descriptor to be treated as non-terminal")
	}
}

func TestRenderMarkdownSkipsEmptyInput(t *testing.T) {
	var buffer bytes.Buffer

	if err := RenderMarkdown(&buffer, "   \n\t"); err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if buffer.Len() != 0 {
		t.Fatalf("expected empty output, got %q", buffer.String())
	}
}

func TestRenderMarkdownWritesTrimmedPlainTextForNonTerminal(t *testing.T) {
	var buffer bytes.Buffer

	if err := RenderMarkdown(&buffer, "  # Heading  "); err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if got := buffer.String(); got != "# Heading\n" {
		t.Fatalf("unexpected rendered output %q", got)
	}
}

func TestRenderMarkdownFallsBackWhenRendererCreationFails(t *testing.T) {
	originalTerminalCheck := terminalWriterCheck
	originalRendererFactory := newMarkdownRenderer
	t.Cleanup(func() {
		terminalWriterCheck = originalTerminalCheck
		newMarkdownRenderer = originalRendererFactory
	})

	terminalWriterCheck = func(io.Writer) bool { return true }
	newMarkdownRenderer = func(io.Writer) (markdownRenderer, error) {
		return nil, errors.New("boom")
	}

	var buffer bytes.Buffer
	if err := RenderMarkdown(&buffer, "  # Heading  "); err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if got := buffer.String(); got != "# Heading\n" {
		t.Fatalf("expected fallback plain text, got %q", got)
	}
}

func TestRenderMarkdownFallsBackWhenRendererRenderFails(t *testing.T) {
	originalTerminalCheck := terminalWriterCheck
	originalRendererFactory := newMarkdownRenderer
	t.Cleanup(func() {
		terminalWriterCheck = originalTerminalCheck
		newMarkdownRenderer = originalRendererFactory
	})

	terminalWriterCheck = func(io.Writer) bool { return true }
	newMarkdownRenderer = func(io.Writer) (markdownRenderer, error) {
		return stubMarkdownRenderer{err: errors.New("render failed")}, nil
	}

	var buffer bytes.Buffer
	if err := RenderMarkdown(&buffer, "  # Heading  "); err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if got := buffer.String(); got != "# Heading\n" {
		t.Fatalf("expected fallback plain text, got %q", got)
	}
}

func TestRenderMarkdownFallsBackWhenRendererPanics(t *testing.T) {
	originalTerminalCheck := terminalWriterCheck
	originalRendererFactory := newMarkdownRenderer
	t.Cleanup(func() {
		terminalWriterCheck = originalTerminalCheck
		newMarkdownRenderer = originalRendererFactory
	})

	terminalWriterCheck = func(io.Writer) bool { return true }
	newMarkdownRenderer = func(io.Writer) (markdownRenderer, error) {
		return panicMarkdownRenderer{}, nil
	}

	var buffer bytes.Buffer
	if err := RenderMarkdown(&buffer, "  # Heading  "); err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if got := buffer.String(); got != "# Heading\n" {
		t.Fatalf("expected fallback plain text after panic, got %q", got)
	}
}

func TestRenderMarkdownWritesRenderedTerminalOutput(t *testing.T) {
	originalTerminalCheck := terminalWriterCheck
	originalRendererFactory := newMarkdownRenderer
	t.Cleanup(func() {
		terminalWriterCheck = originalTerminalCheck
		newMarkdownRenderer = originalRendererFactory
	})

	terminalWriterCheck = func(io.Writer) bool { return true }
	newMarkdownRenderer = func(io.Writer) (markdownRenderer, error) {
		return stubMarkdownRenderer{rendered: "rendered output"}, nil
	}

	var buffer bytes.Buffer
	if err := RenderMarkdown(&buffer, "  # Heading  "); err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	if got := buffer.String(); got != "rendered output" {
		t.Fatalf("unexpected terminal output %q", got)
	}
}

func TestMarkdownStyleOptionHonorsEnvironmentConfig(t *testing.T) {
	t.Setenv("GLAMOUR_STYLE", "dark")

	option := markdownStyleOption(bytes.NewBuffer(nil))
	if _, err := glamour.NewTermRenderer(option); err != nil {
		t.Fatalf("build renderer with environment config: %v", err)
	}
}

func TestMarkdownStyleOptionBuildsRendererForFileWriter(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "markdown-style-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = file.Close() }()

	option := markdownStyleOption(file)
	if _, err := glamour.NewTermRenderer(option); err != nil {
		t.Fatalf("build renderer for file writer: %v", err)
	}
}

func TestMarkdownStyleOptionUsesBackgroundHeuristicForFileWriters(t *testing.T) {
	originalBackgroundCheck := hasDarkBackground
	t.Cleanup(func() { hasDarkBackground = originalBackgroundCheck })

	file, err := os.CreateTemp(t.TempDir(), "markdown-style-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = file.Close() }()

	calls := 0
	hasDarkBackground = func(*os.File) bool {
		calls++
		return false
	}

	option := markdownStyleOption(file)
	if _, err := glamour.NewTermRenderer(option); err != nil {
		t.Fatalf("build renderer for light file writer: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected background heuristic to be used once, got %d", calls)
	}
}

func TestWriterFileDescriptorRequiresFDMethod(t *testing.T) {
	if _, ok := writerFileDescriptor(bytes.NewBuffer(nil)); ok {
		t.Fatal("expected writer without Fd method to be rejected")
	}
}

type stubMarkdownRenderer struct {
	rendered string
	err      error
}

func (s stubMarkdownRenderer) Render(string) (string, error) {
	return s.rendered, s.err
}

type panicMarkdownRenderer struct{}

func (panicMarkdownRenderer) Render(string) (string, error) {
	panic("boom")
}
