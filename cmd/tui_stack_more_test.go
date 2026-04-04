package cmd

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRunTUIUseStackAddsMissingConfigHint(t *testing.T) {
	t.Setenv(configpkg.StackNameEnvVar, configpkg.DefaultStackName)

	var selected string
	withTestDeps(t, func(value *commandDeps) {
		value.setCurrentStackName = func(name string) error {
			selected = name
			return nil
		}
		value.configFilePathForStack = func(name string) (string, error) {
			if name != "staging" {
				t.Fatalf("unexpected stack name: %s", name)
			}
			return "/tmp/stackctl/stacks/staging.yaml", nil
		}
		value.loadConfig = func(string) (configpkg.Config, error) {
			return configpkg.Config{}, configpkg.ErrNotFound
		}
	})

	report, err := runTUIUseStack("staging")
	if err != nil {
		t.Fatalf("runTUIUseStack returned error: %v", err)
	}
	if selected != "staging" {
		t.Fatalf("selected stack = %q", selected)
	}
	if current := os.Getenv(configpkg.StackNameEnvVar); current != "staging" {
		t.Fatalf("expected process stack env to update, got %q", current)
	}
	if !strings.Contains(strings.Join(report.Details, " "), "No config exists yet. Open Config to create it.") {
		t.Fatalf("expected missing-config hint in details: %+v", report.Details)
	}
}

func TestRunTUIUseStackReturnsSelectionAndConfigPathErrors(t *testing.T) {
	t.Run("selection persistence failure", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			value.setCurrentStackName = func(string) error { return errors.New("persist failed") }
		})

		_, err := runTUIUseStack("staging")
		if err == nil || !strings.Contains(err.Error(), "persist failed") {
			t.Fatalf("expected selection persistence failure, got %v", err)
		}
	})

	t.Run("config path failure", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			value.setCurrentStackName = func(string) error { return nil }
			value.configFilePathForStack = func(string) (string, error) { return "", errors.New("path failed") }
		})

		_, err := runTUIUseStack("staging")
		if err == nil || !strings.Contains(err.Error(), "path failed") {
			t.Fatalf("expected config path failure, got %v", err)
		}
	})
}

func TestRunTUIRestartPropagatesComposeRestartFailures(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	t.Run("stack restart down failure", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			value.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
				return errors.New("down failed")
			}
		})

		_, err := runTUIRestart(cfg, nil)
		if err == nil || !strings.Contains(err.Error(), "down failed") {
			t.Fatalf("expected stack restart failure, got %v", err)
		}
	})

	t.Run("service restart up failure", func(t *testing.T) {
		withTestDeps(t, func(value *commandDeps) {
			value.composeUpServices = func(context.Context, system.Runner, configpkg.Config, bool, []string) error {
				return errors.New("service restart failed")
			}
		})

		_, err := runTUIRestart(cfg, []string{"postgres"})
		if err == nil || !strings.Contains(err.Error(), "service restart failed") {
			t.Fatalf("expected service restart failure, got %v", err)
		}
	})
}
