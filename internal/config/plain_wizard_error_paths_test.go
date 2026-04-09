package config

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/creack/pty/v2"

	"github.com/traweezy/stackctl/internal/system"
)

func TestRunWizardErrorPaths(t *testing.T) {
	originalClear := clearWizardScreenFunc
	t.Cleanup(func() { clearWizardScreenFunc = originalClear })
	clearWizardScreenFunc = func(io.Writer) error { return errors.New("clear failed") }

	inMaster, inTTY, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open returned error: %v", err)
	}
	defer func() { _ = inMaster.Close() }()
	defer func() { _ = inTTY.Close() }()

	if _, err := RunWizard(inTTY, inTTY, Default()); err == nil || !strings.Contains(err.Error(), "clear failed") {
		t.Fatalf("expected RunWizard to surface clearWizardScreen errors, got %v", err)
	}
}

func TestRunPlainWizardWithPlatformErrorPaths(t *testing.T) {
	t.Run("managed stack dir failures", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("HOME", "")

		_, err := runPlainWizardWithPlatform(strings.NewReader("\n"), io.Discard, Default(), system.Platform{})
		if err == nil {
			t.Fatal("expected managed stack dir resolution to fail without a data root")
		}
	})

	t.Run("external path read errors", func(t *testing.T) {
		if _, err := runPlainWizardWithPlatform(
			io.MultiReader(strings.NewReader("\nn\n"), &failingLinesReader{err: io.ErrUnexpectedEOF}),
			io.Discard,
			Default(),
			system.Platform{},
		); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected external path read error, got %v", err)
		}
	})

	t.Run("compose file read errors for external stacks", func(t *testing.T) {
		base := minimalPromptCoverageBase()
		base.Stack.Managed = false
		base.Setup.ScaffoldDefaultStack = false
		base.Stack.Dir = t.TempDir()

		if _, err := runPlainWizardWithPlatform(
			io.MultiReader(strings.NewReader(wizardAnswers("", "n", base.Stack.Dir)), &failingLinesReader{err: io.ErrUnexpectedEOF}),
			io.Discard,
			base,
			minimalPromptCoveragePlatform(),
		); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected compose file read error, got %v", err)
		}
	})

	t.Run("startup-timeout read errors", func(t *testing.T) {
		base := minimalPromptCoverageBase()

		if _, err := runPlainWizardWithPlatform(
			io.MultiReader(strings.NewReader(wizardAnswers(
				"", "", "", "", "", "", "", "", "", "",
				"", "", "", "", "", "", "", "",
			)), &failingLinesReader{err: io.ErrUnexpectedEOF}),
			io.Discard,
			base,
			minimalPromptCoveragePlatform(),
		); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected timeout read error, got %v", err)
		}
	})

	t.Run("compose prompt errors", func(t *testing.T) {
		input := io.MultiReader(strings.NewReader("dev-stack\nn\n/tmp/stack\n"), &failingLinesReader{err: io.ErrUnexpectedEOF})
		if _, err := runPlainWizardWithPlatform(input, io.Discard, Default(), system.Platform{}); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected compose prompt read error, got %v", err)
		}
	})

	t.Run("timeout prompt errors", func(t *testing.T) {
		input := io.MultiReader(strings.NewReader(strings.Repeat("\n", 35)), &failingLinesReader{err: io.ErrUnexpectedEOF})
		if _, err := runPlainWizardWithPlatform(input, io.Discard, Default(), system.Platform{}); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected timeout prompt read error, got %v", err)
		}
	})
}
