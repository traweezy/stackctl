//go:build gofuzz

package output

func FuzzRenderMarkdownGo(data []byte) int {
	if len(data) > 1<<14 {
		return 0
	}

	if err := exerciseMarkdownFuzzInput(string(data)); err != nil {
		return 0
	}

	return 1
}
