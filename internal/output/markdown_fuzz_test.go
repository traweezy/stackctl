package output

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

var markdownFuzzSeeds = []string{
	"",
	"   \n\t",
	"# heading",
	"plain text",
	"* list item\n* second item",
	"```go\nfmt.Println(\"hello\")\n```",
	"[link](https://example.com)",
	"[bad-link](javascript:alert(1))",
	"<b>inline html</b>",
	"<script>alert(1)</script>",
	"| h1 | h2 |\n| -- | -- |\n| a | b |",
	"emoji :rocket: and unicode cafe",
}

func FuzzRenderMarkdown(f *testing.F) {
	for _, seed := range markdownFuzzSeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, markdown string) {
		if len(markdown) > 1<<14 {
			t.Skip()
		}

		var buffer bytes.Buffer
		if err := RenderMarkdown(&buffer, markdown); err != nil {
			t.Fatalf("render markdown: %v", err)
		}

		trimmed := strings.TrimSpace(markdown)
		if trimmed == "" {
			if buffer.Len() != 0 {
				t.Fatalf("expected empty output for blank markdown, got %q", buffer.String())
			}
			return
		}

		if got := buffer.String(); got != trimmed+"\n" {
			t.Fatalf("unexpected plain-text output %q", got)
		}

		renderer, err := newMarkdownRenderer(io.Discard)
		if err != nil {
			t.Fatalf("build markdown renderer: %v", err)
		}

		_, _ = renderMarkdownTerminal(renderer, trimmed)
	})
}
