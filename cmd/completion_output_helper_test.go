package cmd

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

type failingWriteBuffer struct {
	bytes.Buffer
	failAfter int
	writes    int
}

func (w *failingWriteBuffer) Write(p []byte) (int, error) {
	w.writes++
	if w.failAfter > 0 && w.writes >= w.failAfter {
		return 0, errors.New("write failed")
	}
	return w.Buffer.Write(p)
}

func TestCompleteExecArgsDelegatesBeforeTheServiceArg(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	completions, directive := completeExecArgs(nil, nil, "s")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected directive: %v", directive)
	}

	choices := completionChoices(completions)
	if !containsChoice(choices, "seaweedfs") {
		t.Fatalf("expected service completion in %v", choices)
	}
	if containsChoice(choices, "pgadmin") {
		t.Fatalf("did not expect disabled pgadmin in %v", choices)
	}
}

func TestCompleteRunArgsDelegatesBeforeDash(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeMeilisearch = true
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	cmd := &cobra.Command{Use: "run"}
	completions, directive := completeRunArgs(cmd, nil, "m")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected directive: %v", directive)
	}

	choices := completionChoices(completions)
	if !containsChoice(choices, "meilisearch") {
		t.Fatalf("expected meilisearch completion in %v", choices)
	}
	if containsChoice(choices, "pgadmin") {
		t.Fatalf("did not expect disabled pgadmin in %v", choices)
	}
}

func TestStackCompletionHelpersStopAfterRequiredArgs(t *testing.T) {
	completions, directive := completeSingleConfiguredStackArg(nil, []string{"staging"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected single-stack directive: %v", directive)
	}
	if len(completions) != 0 {
		t.Fatalf("expected no single-stack completions after the first arg, got %+v", completions)
	}

	completions, directive = completeCloneStackArgs(nil, []string{"staging"}, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected clone directive: %v", directive)
	}
	if len(completions) != 0 {
		t.Fatalf("expected no clone completions after the source arg, got %+v", completions)
	}
}

func TestWithinRootCoversExactNestedAndEscapingPaths(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "stacks", "dev", "compose.yaml")
	if !withinRoot(root, root) {
		t.Fatal("expected root to be within itself")
	}
	if !withinRoot(root, nested) {
		t.Fatalf("expected nested path %q to stay within %q", nested, root)
	}
	if withinRoot(root, filepath.Join(root, "..", "escape")) {
		t.Fatal("expected escaping path to be rejected")
	}
}

func TestLifecycleTargetLabelFormatsLists(t *testing.T) {
	for name, tc := range map[string]struct {
		services []string
		want     string
	}{
		"stack default": {
			services: nil,
			want:     "stack",
		},
		"single known service": {
			services: []string{"postgres"},
			want:     "Postgres",
		},
		"two known services": {
			services: []string{"postgres", "redis"},
			want:     "Postgres and Redis",
		},
		"three mixed services": {
			services: []string{"postgres", "custom", "nats"},
			want:     "Postgres, custom, and NATS",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := lifecycleTargetLabel(tc.services); got != tc.want {
				t.Fatalf("lifecycleTargetLabel(%v) = %q, want %q", tc.services, got, tc.want)
			}
		})
	}
}

func TestWriteConnectionEntriesAndFormatConnectionEntries(t *testing.T) {
	entries := []connectionEntry{
		{Name: "Postgres", Value: "postgres://app"},
		{Name: "Redis", Value: "redis://cache"},
	}

	var out bytes.Buffer
	if err := writeConnectionEntries(&out, entries); err != nil {
		t.Fatalf("writeConnectionEntries returned error: %v", err)
	}

	rendered := out.String()
	for _, fragment := range []string{
		"Postgres\n  postgres://app",
		"\n\nRedis\n  redis://cache",
	} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("rendered entries missing %q:\n%s", fragment, rendered)
		}
	}

	if got := formatConnectionEntries(entries); got != strings.TrimSpace(rendered) {
		t.Fatalf("unexpected formatted entries %q want %q", got, strings.TrimSpace(rendered))
	}

	for _, failAfter := range []int{1, 2} {
		writer := &failingWriteBuffer{failAfter: failAfter}
		if err := writeConnectionEntries(writer, entries); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected writeConnectionEntries to fail after write %d, got %v", failAfter, err)
		}
	}
}

func TestPrintRunDryRunFormatsEnsureRunningMode(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Stack.Name = "stage"

	root := NewRootCmd(NewApp())
	var stdout strings.Builder
	root.SetOut(&stdout)

	err := printRunDryRun(root, cfg, []string{"postgres", "redis"}, []string{"go", "run", "./cmd"}, []envGroup{
		{
			Title: "Postgres",
			Entries: []envEntry{
				{Name: "DATABASE_URL", Value: "postgres://stage"},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("printRunDryRun returned error: %v", err)
	}

	output := stdout.String()
	for _, fragment := range []string{
		"Stack: stage",
		"Services: Postgres, Redis",
		"Service mode: ensure running",
		"Command: 'go' 'run' './cmd'",
		"export DATABASE_URL='postgres://stage'",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected dry-run output to contain %q:\n%s", fragment, output)
		}
	}
}

func TestPrintRunDryRunPropagatesWriterErrors(t *testing.T) {
	cfg := configpkg.Default()

	for _, failAfter := range []int{1, 2, 3, 4, 5} {
		t.Run("fail-after-"+string(rune('0'+failAfter)), func(t *testing.T) {
			root := NewRootCmd(NewApp())
			writer := &failingWriteBuffer{failAfter: failAfter}
			root.SetOut(writer)

			err := printRunDryRun(root, cfg, []string{"postgres"}, []string{"echo", "hi"}, []envGroup{
				{
					Title: "Postgres",
					Entries: []envEntry{
						{Name: "DATABASE_URL", Value: "postgres://app"},
					},
				},
			}, false)
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected printRunDryRun to fail after write %d, got %v", failAfter, err)
			}
		})
	}
}

func TestPrintServicesInfoPropagatesWriterErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
		}
	})

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	for _, failAfter := range []int{1, 2} {
		t.Run("fail-after-"+string(rune('0'+failAfter)), func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetOut(&failingWriteBuffer{failAfter: failAfter})
			if err := printServicesInfo(cmd, cfg); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected printServicesInfo to fail after write %d, got %v", failAfter, err)
			}
		})
	}
}

func TestPrintServicesAndEnvJSONPropagateWriterErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
		}
	})

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	t.Run("services json first write", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})
		if err := printServicesJSON(cmd, cfg); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected printServicesJSON write failure, got %v", err)
		}
	})

	t.Run("services json newline write", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&failingWriteBuffer{failAfter: 2})
		if err := printServicesJSON(cmd, cfg); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected printServicesJSON newline failure, got %v", err)
		}
	})

	t.Run("env json first write", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})
		if err := printEnvJSON(cmd, cfg, []string{"postgres"}); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected printEnvJSON write failure, got %v", err)
		}
	})

	t.Run("env json newline write", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&failingWriteBuffer{failAfter: 2})
		if err := printEnvJSON(cmd, cfg, []string{"postgres"}); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected printEnvJSON newline failure, got %v", err)
		}
	})
}
