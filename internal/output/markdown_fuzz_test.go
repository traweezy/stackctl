package output

import (
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

		if err := exerciseMarkdownFuzzInput(markdown); err != nil {
			t.Fatalf("exercise markdown fuzz input: %v", err)
		}
	})
}
