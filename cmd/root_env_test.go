package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/traweezy/stackctl/internal/logging"
)

func TestVersionTemplateIncludesAppVersion(t *testing.T) {
	app := &App{Version: "1.2.3"}

	if got := versionTemplate(app); got != "stackctl 1.2.3\n" {
		t.Fatalf("unexpected version template %q", got)
	}
}

func TestApplyBoolEnvOverrideHandlesUnsetTrueAndFalse(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("plain", false, "")

	t.Setenv("STACKCTL_TEST_BOOL", "keep")
	if err := applyBoolEnvOverride(cmd, "plain", "STACKCTL_TEST_BOOL"); err != nil {
		t.Fatalf("applyBoolEnvOverride unchanged flag: %v", err)
	}
	if got := os.Getenv("STACKCTL_TEST_BOOL"); got != "keep" {
		t.Fatalf("expected unchanged env to remain set, got %q", got)
	}

	if err := cmd.Flags().Set("plain", "true"); err != nil {
		t.Fatalf("set plain flag true: %v", err)
	}
	if err := applyBoolEnvOverride(cmd, "plain", "STACKCTL_TEST_BOOL"); err != nil {
		t.Fatalf("applyBoolEnvOverride true flag: %v", err)
	}
	if got := os.Getenv("STACKCTL_TEST_BOOL"); got != "1" {
		t.Fatalf("expected true flag to set env, got %q", got)
	}

	other := &cobra.Command{}
	other.Flags().Bool("plain", true, "")
	if err := other.Flags().Set("plain", "false"); err != nil {
		t.Fatalf("set plain flag false: %v", err)
	}
	if err := applyBoolEnvOverride(other, "plain", "STACKCTL_TEST_BOOL"); err != nil {
		t.Fatalf("applyBoolEnvOverride false flag: %v", err)
	}
	if _, ok := os.LookupEnv("STACKCTL_TEST_BOOL"); ok {
		t.Fatal("expected false flag to unset env")
	}

	if err := applyBoolEnvOverride(&cobra.Command{}, "missing", "STACKCTL_TEST_BOOL"); err != nil {
		t.Fatalf("expected missing bool flag to be ignored, got %v", err)
	}
}

func TestApplyBoolEnvOverrideReturnsFlagTypeErrors(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("plain", "", "")
	if err := cmd.Flags().Set("plain", "yes"); err != nil {
		t.Fatalf("set mismatched plain flag: %v", err)
	}

	if err := applyBoolEnvOverride(cmd, "plain", "STACKCTL_TEST_BOOL"); err == nil {
		t.Fatal("expected bool env override to fail for non-bool flag")
	}
}

func TestApplyStringEnvOverrideHandlesUnsetTrimmedAndEmptyValues(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("log-level", "", "")

	t.Setenv("STACKCTL_TEST_STRING", "keep")
	if err := applyStringEnvOverride(cmd, "log-level", "STACKCTL_TEST_STRING"); err != nil {
		t.Fatalf("applyStringEnvOverride unchanged flag: %v", err)
	}
	if got := os.Getenv("STACKCTL_TEST_STRING"); got != "keep" {
		t.Fatalf("expected unchanged env to remain set, got %q", got)
	}

	if err := cmd.Flags().Set("log-level", " debug "); err != nil {
		t.Fatalf("set log-level flag: %v", err)
	}
	if err := applyStringEnvOverride(cmd, "log-level", "STACKCTL_TEST_STRING"); err != nil {
		t.Fatalf("applyStringEnvOverride trimmed value: %v", err)
	}
	if got := os.Getenv("STACKCTL_TEST_STRING"); got != "debug" {
		t.Fatalf("expected trimmed env value, got %q", got)
	}

	other := &cobra.Command{}
	other.Flags().String("log-level", "info", "")
	if err := other.Flags().Set("log-level", "   "); err != nil {
		t.Fatalf("set empty log-level flag: %v", err)
	}
	if err := applyStringEnvOverride(other, "log-level", "STACKCTL_TEST_STRING"); err != nil {
		t.Fatalf("applyStringEnvOverride empty value: %v", err)
	}
	if _, ok := os.LookupEnv("STACKCTL_TEST_STRING"); ok {
		t.Fatal("expected empty trimmed value to unset env")
	}

	if err := applyStringEnvOverride(&cobra.Command{}, "missing", "STACKCTL_TEST_STRING"); err != nil {
		t.Fatalf("expected missing string flag to be ignored, got %v", err)
	}
}

func TestApplyStringEnvOverrideReturnsFlagTypeErrors(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("log-level", false, "")
	if err := cmd.Flags().Set("log-level", "true"); err != nil {
		t.Fatalf("set mismatched log-level flag: %v", err)
	}

	if err := applyStringEnvOverride(cmd, "log-level", "STACKCTL_TEST_STRING"); err == nil {
		t.Fatal("expected string env override to fail for non-string flag")
	}
}

func TestApplyRootEnvOverridesUpdatesAllSupportedEnvironmentVariables(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("accessible", false, "")
	cmd.Flags().Bool("plain", false, "")
	cmd.Flags().String("log-level", "", "")
	cmd.Flags().String("log-format", "", "")
	cmd.Flags().String("log-file", "", "")

	t.Setenv("ACCESSIBLE", "")
	t.Setenv("STACKCTL_WIZARD_PLAIN", "1")
	t.Setenv(logging.EnvLogLevel, "")
	t.Setenv(logging.EnvLogFormat, "")
	t.Setenv(logging.EnvLogFile, "stale")

	if err := cmd.Flags().Set("accessible", "true"); err != nil {
		t.Fatalf("set accessible: %v", err)
	}
	if err := cmd.Flags().Set("plain", "false"); err != nil {
		t.Fatalf("set plain: %v", err)
	}
	if err := cmd.Flags().Set("log-level", "warn"); err != nil {
		t.Fatalf("set log-level: %v", err)
	}
	if err := cmd.Flags().Set("log-format", "json"); err != nil {
		t.Fatalf("set log-format: %v", err)
	}
	if err := cmd.Flags().Set("log-file", " /tmp/stackctl.log "); err != nil {
		t.Fatalf("set log-file: %v", err)
	}

	if err := applyRootEnvOverrides(cmd); err != nil {
		t.Fatalf("applyRootEnvOverrides returned error: %v", err)
	}

	if got := os.Getenv("ACCESSIBLE"); got != "1" {
		t.Fatalf("expected ACCESSIBLE=1, got %q", got)
	}
	if _, ok := os.LookupEnv("STACKCTL_WIZARD_PLAIN"); ok {
		t.Fatal("expected plain override to unset STACKCTL_WIZARD_PLAIN")
	}
	if got := os.Getenv(logging.EnvLogLevel); got != "warn" {
		t.Fatalf("expected log level override, got %q", got)
	}
	if got := os.Getenv(logging.EnvLogFormat); got != "json" {
		t.Fatalf("expected log format override, got %q", got)
	}
	if got := os.Getenv(logging.EnvLogFile); got != "/tmp/stackctl.log" {
		t.Fatalf("expected trimmed log file override, got %q", got)
	}
}

func TestApplyRootEnvOverridesReturnsFirstOverrideError(t *testing.T) {
	t.Run("bool override failure stops processing", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("accessible", "", "")
		cmd.Flags().Bool("plain", false, "")
		cmd.Flags().String("log-level", "", "")
		cmd.Flags().String("log-format", "", "")
		cmd.Flags().String("log-file", "", "")

		if err := cmd.Flags().Set("accessible", "yes"); err != nil {
			t.Fatalf("set accessible: %v", err)
		}
		if err := applyRootEnvOverrides(cmd); err == nil {
			t.Fatal("expected applyRootEnvOverrides to fail on mismatched bool flag")
		}
	})

	t.Run("string override failure is surfaced after bool overrides", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("accessible", false, "")
		cmd.Flags().Bool("plain", false, "")
		cmd.Flags().Bool("log-level", false, "")
		cmd.Flags().String("log-format", "", "")
		cmd.Flags().String("log-file", "", "")

		t.Setenv("ACCESSIBLE", "")
		t.Setenv("STACKCTL_WIZARD_PLAIN", "1")
		if err := cmd.Flags().Set("accessible", "true"); err != nil {
			t.Fatalf("set accessible: %v", err)
		}
		if err := cmd.Flags().Set("plain", "false"); err != nil {
			t.Fatalf("set plain: %v", err)
		}
		if err := cmd.Flags().Set("log-level", "true"); err != nil {
			t.Fatalf("set log-level: %v", err)
		}

		if err := applyRootEnvOverrides(cmd); err == nil {
			t.Fatal("expected applyRootEnvOverrides to fail on mismatched string flag")
		}
		if got := os.Getenv("ACCESSIBLE"); got != "1" {
			t.Fatalf("expected earlier bool override to apply before failure, got %q", got)
		}
		if _, ok := os.LookupEnv("STACKCTL_WIZARD_PLAIN"); ok {
			t.Fatal("expected earlier plain override to unset env before failure")
		}
	})
}

func TestApplyRootEnvOverridesReturnsErrorsFromRemainingOverrideStages(t *testing.T) {
	tests := []struct {
		name     string
		setupCmd func(*cobra.Command) error
	}{
		{
			name: "plain override failure",
			setupCmd: func(cmd *cobra.Command) error {
				cmd.Flags().Bool("accessible", false, "")
				cmd.Flags().String("plain", "", "")
				cmd.Flags().String("log-level", "", "")
				cmd.Flags().String("log-format", "", "")
				cmd.Flags().String("log-file", "", "")
				if err := cmd.Flags().Set("accessible", "true"); err != nil {
					return err
				}
				return cmd.Flags().Set("plain", "no")
			},
		},
		{
			name: "log format override failure",
			setupCmd: func(cmd *cobra.Command) error {
				cmd.Flags().Bool("accessible", false, "")
				cmd.Flags().Bool("plain", false, "")
				cmd.Flags().String("log-level", "", "")
				cmd.Flags().Bool("log-format", false, "")
				cmd.Flags().String("log-file", "", "")
				if err := cmd.Flags().Set("accessible", "true"); err != nil {
					return err
				}
				if err := cmd.Flags().Set("plain", "false"); err != nil {
					return err
				}
				if err := cmd.Flags().Set("log-level", "warn"); err != nil {
					return err
				}
				return cmd.Flags().Set("log-format", "true")
			},
		},
		{
			name: "log file override failure",
			setupCmd: func(cmd *cobra.Command) error {
				cmd.Flags().Bool("accessible", false, "")
				cmd.Flags().Bool("plain", false, "")
				cmd.Flags().String("log-level", "", "")
				cmd.Flags().String("log-format", "", "")
				cmd.Flags().Bool("log-file", false, "")
				if err := cmd.Flags().Set("accessible", "true"); err != nil {
					return err
				}
				if err := cmd.Flags().Set("plain", "false"); err != nil {
					return err
				}
				if err := cmd.Flags().Set("log-level", "warn"); err != nil {
					return err
				}
				if err := cmd.Flags().Set("log-format", "json"); err != nil {
					return err
				}
				return cmd.Flags().Set("log-file", "true")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			if err := tt.setupCmd(cmd); err != nil {
				t.Fatalf("setup command flags: %v", err)
			}
			if err := applyRootEnvOverrides(cmd); err == nil {
				t.Fatal("expected applyRootEnvOverrides to surface stage error")
			}
		})
	}
}
