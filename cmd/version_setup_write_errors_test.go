package cmd

import (
	"io"
	"strings"
	"testing"
)

func TestVersionCommandPropagatesWriterErrors(t *testing.T) {
	app := NewApp()
	app.GitCommit = "abc123"
	app.BuildDate = "2026-03-21"

	t.Run("plain first line", func(t *testing.T) {
		root := NewRootCmd(app)
		root.SetOut(&failingWriteBuffer{failAfter: 1})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"version"})

		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected plain version write failure, got %v", err)
		}
	})

	t.Run("plain git commit line", func(t *testing.T) {
		root := NewRootCmd(app)
		root.SetOut(&failingWriteBuffer{failAfter: 2})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"version"})

		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected git commit write failure, got %v", err)
		}
	})

	t.Run("plain build date line", func(t *testing.T) {
		root := NewRootCmd(app)
		root.SetOut(&failingWriteBuffer{failAfter: 3})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"version"})

		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected build date write failure, got %v", err)
		}
	})

	t.Run("json body write", func(t *testing.T) {
		root := NewRootCmd(app)
		root.SetOut(&failingWriteBuffer{failAfter: 1})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"version", "--json"})

		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected version json write failure, got %v", err)
		}
	})

	t.Run("json newline write", func(t *testing.T) {
		root := NewRootCmd(app)
		root.SetOut(&failingWriteBuffer{failAfter: 2})
		root.SetErr(io.Discard)
		root.SetArgs([]string{"version", "--json"})

		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected version json newline failure, got %v", err)
		}
	})
}

func TestSetupPropagatesConfigMissingStatusWriteErrors(t *testing.T) {
	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"setup", "--non-interactive"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected setup status write failure, got %v", err)
	}
}
