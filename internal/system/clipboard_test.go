package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClipboardCommandPrefersXclipWhenWaylandIsUnavailable(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "wl-copy"))
	writeExecutable(t, filepath.Join(binDir, "xclip"))

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_SESSION_TYPE", "x11")

	command, args, ok := clipboardCommand()
	if !ok {
		t.Fatal("expected clipboard command to be detected")
	}
	if command != "xclip" {
		t.Fatalf("expected xclip fallback, got %q", command)
	}
	if len(args) != 2 || args[0] != "-selection" || args[1] != "clipboard" {
		t.Fatalf("unexpected xclip args: %+v", args)
	}
}

func TestClipboardCommandUsesWaylandClipboardWhenSessionIsWayland(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "wl-copy"))
	writeExecutable(t, filepath.Join(binDir, "xclip"))

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("XDG_SESSION_TYPE", "wayland")

	command, args, ok := clipboardCommand()
	if !ok {
		t.Fatal("expected clipboard command to be detected")
	}
	if command != "wl-copy" {
		t.Fatalf("expected wl-copy, got %q", command)
	}
	if len(args) != 0 {
		t.Fatalf("expected wl-copy args to be empty, got %+v", args)
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}
