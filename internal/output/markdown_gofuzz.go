//go:build gofuzz

package output

import (
	"bytes"
	"io"
	"strings"
)

func FuzzRenderMarkdownGo(data []byte) int {
	if len(data) > 1<<14 {
		return 0
	}

	var buffer bytes.Buffer
	markdown := string(data)
	if err := RenderMarkdown(&buffer, markdown); err != nil {
		return 0
	}

	trimmed := strings.TrimSpace(markdown)
	if trimmed == "" {
		return 1
	}

	renderer, err := newMarkdownRenderer(io.Discard)
	if err != nil {
		return 0
	}

	_, _ = renderMarkdownTerminal(renderer, trimmed)
	return 1
}
