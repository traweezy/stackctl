package output

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

func exerciseMarkdownFuzzInput(markdown string) error {
	var buffer bytes.Buffer
	if err := RenderMarkdown(&buffer, markdown); err != nil {
		return err
	}

	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" {
		if buffer.Len() != 0 {
			return fmt.Errorf("expected empty output for blank markdown, got %q", buffer.String())
		}
		return nil
	}

	if got := buffer.String(); got != trimmed+"\n" {
		return fmt.Errorf("unexpected plain-text output %q", got)
	}

	renderer, err := newMarkdownRenderer(io.Discard)
	if err != nil {
		return err
	}

	_, _ = renderMarkdownTerminal(renderer, trimmed)
	return nil
}
