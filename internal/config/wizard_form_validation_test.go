package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	huh "charm.land/huh/v2"

	"github.com/traweezy/stackctl/internal/system"
)

func TestBuildWizardFormValidationEdgeCases(t *testing.T) {
	platform := system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	}

	t.Run("missing path description falls back when abs resolution fails", func(t *testing.T) {
		originalWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd returned error: %v", err)
		}
		badCWD := t.TempDir()
		if err := os.Chdir(badCWD); err != nil {
			t.Fatalf("Chdir returned error: %v", err)
		}
		if err := os.RemoveAll(badCWD); err != nil {
			t.Fatalf("RemoveAll returned error: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(originalWD)
		})

		state := newWizardState(DefaultForStackOnPlatform("dev-stack", platform))
		state.StackMode = wizardStackModeExternal
		state.ExternalStackDir = "missing-stack"
		form := buildWizardForm(&state, platform)
		groups := wizardGroupsByTitle(t, form)
		fields := wizardGroupFields(t, groups["External Path"])
		confirm, ok := fields[1].(*huh.Confirm)
		if !ok {
			t.Fatalf("expected confirm field, got %T", fields[1])
		}

		description := wizardEvalStringFunc(t, confirm, "description")
		if got := description(); !strings.Contains(got, "Choose an existing directory") {
			t.Fatalf("expected fallback description, got %q", got)
		}
	})

	t.Run("external path validator rejects unchecked missing directories", func(t *testing.T) {
		state := newWizardState(DefaultForStackOnPlatform("dev-stack", platform))
		state.StackMode = wizardStackModeExternal
		state.ExternalStackDir = filepath.Join(t.TempDir(), "missing-stack")
		form := buildWizardForm(&state, platform)
		groups := wizardGroupsByTitle(t, form)
		fields := wizardGroupFields(t, groups["External Path"])
		confirm, ok := fields[1].(*huh.Confirm)
		if !ok {
			t.Fatalf("expected confirm field, got %T", fields[1])
		}

		validate := wizardHiddenField(t, reflect.ValueOf(confirm), "validate")
		if err := validate.Interface().(func(bool) error)(false); err == nil || !strings.Contains(err.Error(), "choose an existing directory") {
			t.Fatalf("expected missing-dir validation error, got %v", err)
		}
	})

	t.Run("services validator rejects empty selections", func(t *testing.T) {
		state := newWizardState(DefaultForStackOnPlatform("dev-stack", platform))
		state.Services = nil
		form := buildWizardForm(&state, platform)
		groups := wizardGroupsByTitle(t, form)
		fields := wizardGroupFields(t, groups["Services"])
		selector, ok := fields[1].(*huh.MultiSelect[string])
		if !ok {
			t.Fatalf("expected service selector, got %T", fields[1])
		}

		validate := wizardHiddenField(t, reflect.ValueOf(selector), "validate")
		if err := validate.Interface().(func([]string) error)(nil); err == nil || !strings.Contains(err.Error(), "select at least one stack service") {
			t.Fatalf("expected service validation error, got %v", err)
		}
	})

	t.Run("external path confirm validator accepts explicit confirmation", func(t *testing.T) {
		state := newWizardState(DefaultForStackOnPlatform("dev-stack", platform))
		state.StackMode = wizardStackModeExternal
		state.ExternalStackDir = filepath.Join(t.TempDir(), "missing-stack")

		form := buildWizardForm(&state, platform)
		confirm, ok := wizardGroupFields(t, wizardGroupsByTitle(t, form)["External Path"])[1].(*huh.Confirm)
		if !ok {
			t.Fatalf("expected confirm field, got %T", wizardGroupFields(t, wizardGroupsByTitle(t, form)["External Path"])[1])
		}

		validate := wizardHiddenField(t, reflect.ValueOf(confirm), "validate")
		if err := validate.Interface().(func(bool) error)(true); err != nil {
			t.Fatalf("expected confirmation validator to accept true, got %v", err)
		}
	})

	t.Run("needsMissingExternalDirConfirmation ignores blank external paths", func(t *testing.T) {
		state := newWizardState(DefaultForStackOnPlatform("dev-stack", platform))
		state.StackMode = wizardStackModeExternal
		state.ExternalStackDir = "   "
		if state.needsMissingExternalDirConfirmation() {
			t.Fatal("expected blank external paths to skip confirmation")
		}
	})

	t.Run("Validate rejects whitespace in redis ACL usernames", func(t *testing.T) {
		cfg := Default()
		cfg.Connection.RedisACLUsername = "stack user"
		cfg.Connection.RedisACLPassword = "stack-pass"

		issues := Validate(cfg)
		for _, issue := range issues {
			if issue.Field == "connection.redis_acl_username" && strings.Contains(issue.Message, "must not contain whitespace") {
				return
			}
		}
		t.Fatalf("expected redis ACL username whitespace validation issue, got %+v", issues)
	})
}
