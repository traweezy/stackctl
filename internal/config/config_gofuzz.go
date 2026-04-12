//go:build gofuzz

package config

func FuzzConfigLoadAndRenderGo(data []byte) int {
	if len(data) > 1<<16 {
		return 0
	}

	if !exerciseConfigFuzzInput(data) {
		return 0
	}

	return 1
}
