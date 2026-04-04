package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestConfirmWithPromptDelegatesToPromptHandler(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.stdin = strings.NewReader("y\n")
		d.promptYesNo = func(in io.Reader, out io.Writer, question string, defaultYes bool) (bool, error) {
			if question != "Continue?" {
				t.Fatalf("unexpected prompt question %q", question)
			}
			if !defaultYes {
				t.Fatal("expected defaultYes to be forwarded")
			}
			if in == nil || out == nil {
				t.Fatal("expected prompt IO to be forwarded")
			}
			return true, nil
		}
	})

	cmd := &cobra.Command{Use: "support"}
	var out bytes.Buffer
	cmd.SetOut(&out)

	ok, err := confirmWithPrompt(cmd, "Continue?", true)
	if err != nil {
		t.Fatalf("confirmWithPrompt returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected confirmation to return true")
	}
}

func TestUserCancelledAndValidationIssuePrintingProduceExpectedStatusOutput(t *testing.T) {
	cmd := &cobra.Command{Use: "support"}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := userCancelled(cmd, "database reset cancelled"); err != nil {
		t.Fatalf("userCancelled returned error: %v", err)
	}

	issues := []configpkg.ValidationIssue{
		{Field: "stack.name", Message: "must not be empty"},
		{Field: "ports.postgres", Message: "must be between 1 and 65535"},
	}
	if err := printValidationIssues(cmd, issues); err != nil {
		t.Fatalf("printValidationIssues returned error: %v", err)
	}

	text := out.String()
	for _, fragment := range []string{
		"database reset cancelled",
		"stack.name: must not be empty",
		"ports.postgres: must be between 1 and 65535",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected status output to contain %q:\n%s", fragment, text)
		}
	}
}

func TestPendingManagedScaffoldIssueMatchesOnlyExpectedManagedDefaults(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.ApplyDerivedFields()

	dirIssue := configpkg.ValidationIssue{
		Field:   "stack.dir",
		Message: "directory does not exist: " + cfg.Stack.Dir,
	}
	if !pendingManagedScaffoldIssue(cfg, dirIssue) {
		t.Fatalf("expected managed scaffold dir issue to be filtered: %+v", dirIssue)
	}

	composeIssue := configpkg.ValidationIssue{
		Field:   "stack.compose_file",
		Message: "file does not exist: " + configpkg.ComposePath(cfg),
	}
	if !pendingManagedScaffoldIssue(cfg, composeIssue) {
		t.Fatalf("expected managed scaffold compose issue to be filtered: %+v", composeIssue)
	}

	if pendingManagedScaffoldIssue(cfg, configpkg.ValidationIssue{Field: "stack.dir", Message: "directory does not exist: /tmp/other"}) {
		t.Fatal("expected mismatched dir issue to remain visible")
	}

	unmanaged := cfg
	unmanaged.Stack.Managed = false
	if pendingManagedScaffoldIssue(unmanaged, dirIssue) {
		t.Fatal("expected unmanaged stacks to keep dir issues")
	}

	noScaffold := cfg
	noScaffold.Setup.ScaffoldDefaultStack = false
	if pendingManagedScaffoldIssue(noScaffold, composeIssue) {
		t.Fatal("expected non-scaffolded managed stacks to keep compose issues")
	}

	invalidName := cfg
	invalidName.Stack.Name = "INVALID!"
	if pendingManagedScaffoldIssue(invalidName, dirIssue) {
		t.Fatal("expected invalid managed stack names to keep dir issues")
	}
}
