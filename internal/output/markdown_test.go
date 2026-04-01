package output

import "testing"

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
