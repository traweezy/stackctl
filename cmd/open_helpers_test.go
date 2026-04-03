package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDefaultOpenTargetPrefersEnabledServicesInPriorityOrder(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Setup.IncludeCockpit = true
	cfg.Setup.IncludeMeilisearch = true
	cfg.Setup.IncludePgAdmin = true

	if got := defaultOpenTarget(cfg); got != "cockpit" {
		t.Fatalf("expected cockpit default target, got %q", got)
	}

	cfg.Setup.IncludeCockpit = false
	if got := defaultOpenTarget(cfg); got != "meilisearch" {
		t.Fatalf("expected meilisearch default target, got %q", got)
	}

	cfg.Setup.IncludeMeilisearch = false
	if got := defaultOpenTarget(cfg); got != "pgadmin" {
		t.Fatalf("expected pgadmin default target, got %q", got)
	}

	cfg.Setup.IncludePgAdmin = false
	if got := defaultOpenTarget(cfg); got != "cockpit" {
		t.Fatalf("expected cockpit fallback target, got %q", got)
	}
}

func TestOpenConfiguredURLHandlesSuccessfulAndFallbackLaunches(t *testing.T) {
	originalOutput := rootOutput
	rootOutput = rootOutputOptions{}
	t.Cleanup(func() {
		rootOutput = originalOutput
	})

	t.Run("open succeeds", func(t *testing.T) {
		var opened string
		withTestDeps(t, func(d *commandDeps) {
			d.openURL = func(_ context.Context, _ system.Runner, target string) error {
				opened = target
				return nil
			}
		})

		cmd := &cobra.Command{}
		cmd.Flags().Bool("quiet", false, "")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)

		if err := openConfiguredURL(cmd, "pgadmin", "http://127.0.0.1:5050"); err != nil {
			t.Fatalf("openConfiguredURL returned error: %v", err)
		}
		if opened != "http://127.0.0.1:5050" {
			t.Fatalf("expected target to be opened, got %q", opened)
		}
		if stdout.Len() != 0 {
			t.Fatalf("expected no fallback output, got %q", stdout.String())
		}
	})

	t.Run("open falls back to warning and url", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.openURL = func(context.Context, system.Runner, string) error {
				return errors.New("browser unavailable")
			}
		})

		cmd := &cobra.Command{}
		cmd.Flags().Bool("quiet", false, "")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)

		if err := openConfiguredURL(cmd, "pgadmin", "http://127.0.0.1:5050"); err != nil {
			t.Fatalf("openConfiguredURL returned error: %v", err)
		}
		output := stdout.String()
		if !strings.Contains(output, "could not open pgadmin automatically; use this URL") {
			t.Fatalf("expected fallback warning, got %q", output)
		}
		if !strings.Contains(output, "http://127.0.0.1:5050") {
			t.Fatalf("expected fallback URL, got %q", output)
		}
	})
}
