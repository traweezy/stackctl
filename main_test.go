package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestMainRunsVersionCommand(t *testing.T) {
	originalArgs := os.Args
	originalStdout := os.Stdout

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	os.Args = []string{"stackctl", "version"}
	os.Stdout = writePipe

	t.Cleanup(func() {
		os.Args = originalArgs
		os.Stdout = originalStdout
	})

	main()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}

	output, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	if !strings.Contains(string(output), version) {
		t.Fatalf("unexpected main output: %q", string(output))
	}
}
