package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

const testMainModeEnv = "STACKCTL_TEST_MAIN_MODE"

func TestMainPrintsVersionJSON(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess")
	cmd.Env = append(os.Environ(), testMainModeEnv+"=version-json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("run main helper: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}

	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse version json: %v", err)
	}
	if payload.Version != version {
		t.Fatalf("expected version %q, got %q", version, payload.Version)
	}
}

func TestMainExitsNonZeroOnCommandError(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelperProcess")
	cmd.Env = append(os.Environ(), testMainModeEnv+"=invalid-flag")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected main helper to fail")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected exit error, got %T", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected command error on stderr")
	}
}

func TestMainHelperProcess(t *testing.T) {
	mode := os.Getenv(testMainModeEnv)
	if mode == "" {
		return
	}

	switch mode {
	case "version-json":
		os.Args = []string{"stackctl", "version", "--json"}
	case "invalid-flag":
		os.Args = []string{"stackctl", "--definitely-invalid"}
	default:
		t.Fatalf("unexpected helper mode %q", mode)
	}

	main()
	os.Exit(0)
}
