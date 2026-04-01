package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	glamour "charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

func RenderMarkdown(w io.Writer, markdown string) error {
	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" {
		return nil
	}

	if !isTerminalWriter(w) {
		_, err := fmt.Fprintln(w, trimmed)
		return err
	}

	renderer, err := glamour.NewTermRenderer(
		markdownStyleOption(w),
		glamour.WithWordWrap(100),
		glamour.WithEmoji(),
	)
	if err != nil {
		_, writeErr := fmt.Fprintln(w, trimmed)
		return writeErr
	}

	rendered, err := renderer.Render(trimmed)
	if err != nil {
		_, writeErr := fmt.Fprintln(w, trimmed)
		return writeErr
	}

	_, err = io.WriteString(w, rendered)
	return err
}

func markdownStyleOption(w io.Writer) glamour.TermRendererOption {
	if strings.TrimSpace(os.Getenv("GLAMOUR_STYLE")) != "" {
		return glamour.WithEnvironmentConfig()
	}
	if file, ok := w.(*os.File); ok && !lipgloss.HasDarkBackground(os.Stdin, file) {
		return glamour.WithStandardStyle("light")
	}
	return glamour.WithStandardStyle("dark")
}

func isTerminalWriter(w io.Writer) bool {
	fd, ok := writerFileDescriptor(w)
	if !ok {
		return false
	}

	return term.IsTerminal(fd)
}

func writerFileDescriptor(w io.Writer) (int, bool) {
	value, ok := w.(interface{ Fd() uintptr })
	if !ok {
		return 0, false
	}

	fd := value.Fd()
	maxInt := ^uint(0) >> 1
	if fd > uintptr(maxInt) {
		return 0, false
	}

	// #nosec G115 -- fd is range-checked against the platform int size above.
	return int(fd), true
}
