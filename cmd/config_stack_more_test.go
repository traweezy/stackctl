package cmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestConfigEditReturnsMissingConfigHint(t *testing.T) {
	withTestDeps(t, nil)

	_, _, err := executeRoot(t, "config", "edit", "--non-interactive")
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigEditPropagatesSaveErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.saveConfig = func(string, configpkg.Config) error { return errors.New("save failed") }
	})

	_, _, err := executeRoot(t, "config", "edit", "--non-interactive")
	if err == nil || !strings.Contains(err.Error(), "save failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigEditPropagatesOutputWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"config", "edit", "--non-interactive"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected config edit write failure, got %v", err)
	}
}

func TestStackUsePropagatesConfigPathErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.configFilePathForStack = func(string) (string, error) { return "", errors.New("path failed") }
	})

	_, _, err := executeRoot(t, "stack", "use", "staging")
	if err == nil || !strings.Contains(err.Error(), "path failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStackUsePropagatesExistingConfigLoadErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.configFilePathForStack = func(name string) (string, error) {
			return "/tmp/stackctl/stacks/" + name + ".yaml", nil
		}
		d.stat = func(path string) (os.FileInfo, error) {
			if path == "/tmp/stackctl/stacks/staging.yaml" {
				return fakeFileInfo{name: filepath.Base(path)}, nil
			}
			return nil, os.ErrNotExist
		}
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("load failed") }
	})

	_, _, err := executeRoot(t, "stack", "use", "staging")
	if err == nil || !strings.Contains(err.Error(), "load failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStackUsePropagatesStatusLineWriteErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, configpkg.ErrNotFound }
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 1})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"stack", "use", "staging"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected stack use status write failure, got %v", err)
	}
}
