package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestLoadCompletionConfigReturnsSuccessAndFailure(t *testing.T) {
	withTestDeps(t, nil)

	if _, ok := loadCompletionConfig(); ok {
		t.Fatal("expected missing config to disable completion config loading")
	}

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Stack.Name = "staging"
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	cfg, ok := loadCompletionConfig()
	if !ok {
		t.Fatal("expected completion config to load")
	}
	if cfg.Stack.Name != "staging" {
		t.Fatalf("unexpected loaded config: %+v", cfg.Stack)
	}
}

func TestCompletionServiceDefinitionsFallsBackAndUsesEnabledConfig(t *testing.T) {
	withTestDeps(t, nil)

	all := completionServiceDefinitions()
	if len(all) != len(serviceDefinitions()) {
		t.Fatalf("expected fallback to all service definitions, got %d want %d", len(all), len(serviceDefinitions()))
	}

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeSeaweedFS = true
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	enabled := completionServiceDefinitions()
	names := make([]string, 0, len(enabled))
	for _, definition := range enabled {
		names = append(names, definition.Key)
	}
	if containsChoice(names, "pgadmin") {
		t.Fatalf("did not expect disabled pgadmin in %+v", names)
	}
	if !containsChoice(names, "seaweedfs") {
		t.Fatalf("expected enabled seaweedfs in %+v", names)
	}
}

func TestNamesWithDescriptionsLeavesUnknownDescriptionsBare(t *testing.T) {
	completions := namesWithDescriptions([]string{"alpha", "beta"}, map[string]string{
		"beta": "configured stack",
	})
	choice := string(completions[0])
	if strings.Contains(choice, "\t") {
		t.Fatalf("expected bare completion for alpha, got %q", choice)
	}
	if !strings.Contains(string(completions[1]), "\tconfigured stack") {
		t.Fatalf("expected described completion for beta, got %q", string(completions[1]))
	}
}

func TestCompleteOpenTargetsFallsBackWhenConfigUnavailable(t *testing.T) {
	withTestDeps(t, nil)

	completions, directive := completeOpenTargets(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("unexpected directive: %v", directive)
	}
	choices := completionChoices(completions)
	for _, expected := range []string{"cockpit", "meilisearch", "pgadmin", "all"} {
		if !containsChoice(choices, expected) {
			t.Fatalf("expected fallback open target %q in %v", expected, choices)
		}
	}
}

func TestOpenConfiguredURLPropagatesWriterErrors(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.openURL = func(context.Context, system.Runner, string) error {
			return errors.New("browser unavailable")
		}
	})

	t.Run("warning write", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("quiet", false, "")
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})
		cmd.SetErr(bytes.NewBuffer(nil))

		if err := openConfiguredURL(cmd, "pgadmin", "http://127.0.0.1:5050"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected warning write failure, got %v", err)
		}
	})

	t.Run("url write", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("quiet", false, "")
		cmd.SetOut(&failingWriteBuffer{failAfter: 2})
		cmd.SetErr(bytes.NewBuffer(nil))

		if err := openConfiguredURL(cmd, "pgadmin", "http://127.0.0.1:5050"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected URL write failure, got %v", err)
		}
	})
}

func TestPrintDoctorReportPropagatesWriterErrors(t *testing.T) {
	t.Run("status line write", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})

		report := newReport(doctorpkg.Check{Status: output.StatusWarn, Message: "warning"})
		if err := printDoctorReport(cmd, report); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected status line write failure, got %v", err)
		}
	})

	t.Run("summary write", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return false }
		})

		cmd := &cobra.Command{}
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})

		if err := printDoctorReport(cmd, doctorpkg.Report{}); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected summary write failure, got %v", err)
		}
	})
}
