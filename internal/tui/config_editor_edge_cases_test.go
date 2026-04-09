package tui

import (
	"errors"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestConfigEditorEdgeCases(t *testing.T) {
	t.Run("applyPlan returns manual compose updates for managed stacks with scaffold disabled", func(t *testing.T) {
		originalAnalyze := configEditorAnalyzeConfigImpact
		t.Cleanup(func() { configEditorAnalyzeConfigImpact = originalAnalyze })
		configEditorAnalyzeConfigImpact = func(configpkg.Config, configpkg.Config) configImpact {
			return configImpact{changed: true, composeTemplate: true}
		}

		editor := newConfigEditor()
		editor.baseline = configpkg.Default()
		editor.draft = configpkg.Default()
		editor.draft.Setup.ScaffoldDefaultStack = false
		editor.draft.ApplyDerivedFields()

		plan := editor.applyPlan()
		if plan.Allowed || plan.Reason != "managed compose changes require manual compose updates" {
			t.Fatalf("unexpected plan: %+v", plan)
		}
	})

	t.Run("wide view returns the body directly when the status strip is omitted", func(t *testing.T) {
		editor := newConfigEditor()
		editor.baseline = configpkg.Default()
		editor.draft = editor.baseline
		editor.width = 120
		editor.height = 8
		editor.refreshList(false)

		view := stripANSITest(editor.View(false))
		if strings.Contains(view, "Status") {
			t.Fatalf("expected compact wide view to omit the status strip, got %q", view)
		}
		if !strings.Contains(view, "Config fields") {
			t.Fatalf("expected view body to remain visible, got %q", view)
		}
	})

	t.Run("renderPreview and diffText surface injected marshal failures", func(t *testing.T) {
		originalMarshalConfig := configEditorMarshalConfig
		t.Cleanup(func() { configEditorMarshalConfig = originalMarshalConfig })

		editor := newConfigEditor()
		editor.source = ConfigSourceLoaded
		editor.baseline = configpkg.Default()
		editor.draft = editor.baseline

		oldErr := errors.New("old marshal failed")
		callCount := 0
		configEditorMarshalConfig = func(configpkg.Config) ([]byte, error) {
			callCount++
			if callCount == 1 {
				return nil, oldErr
			}
			return []byte("ok"), nil
		}
		if _, err := editor.diffText(false); !errors.Is(err, oldErr) {
			t.Fatalf("expected old diff marshal error %v, got %v", oldErr, err)
		}

		newErr := errors.New("new marshal failed")
		editor.source = ConfigSourceUnavailable
		configEditorMarshalConfig = func(configpkg.Config) ([]byte, error) {
			return nil, newErr
		}
		if _, err := editor.diffText(false); !errors.Is(err, newErr) {
			t.Fatalf("expected new diff marshal error %v, got %v", newErr, err)
		}

		preview := stripANSITest(editor.renderPreview(false))
		if !strings.Contains(preview, "new marshal failed") {
			t.Fatalf("expected preview to surface marshal error, got %q", preview)
		}
	})

	t.Run("applyConfigCmd reports an up-to-date scaffold when the result message is blank", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		manager := configTestManager()
		manager.ScaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
			return configpkg.ScaffoldResult{}, nil
		}

		msg := msgOfType[configOperationMsg](t, applyConfigCmd(&manager, nil, "/tmp/stackctl/config.yaml", cfg, cfg, configApplyPlan{
			Allowed:  true,
			Scaffold: true,
		}))
		if msg.Status != output.StatusOK || !msg.Reload || !strings.Contains(msg.Message, "managed stack scaffold is up to date") {
			t.Fatalf("unexpected apply scaffold message: %+v", msg)
		}
	})

	t.Run("saveFollowUpMessage covers the review fallback via injected impact analysis", func(t *testing.T) {
		originalAnalyze := configEditorAnalyzeConfigImpact
		t.Cleanup(func() { configEditorAnalyzeConfigImpact = originalAnalyze })
		configEditorAnalyzeConfigImpact = func(configpkg.Config, configpkg.Config) configImpact {
			return configImpact{changed: true}
		}

		if got := saveFollowUpMessage(configpkg.Default(), configpkg.Default(), 0); got != "review the config impact before the next restart" {
			t.Fatalf("unexpected fallback save follow-up: %q", got)
		}
	})

	t.Run("classifyConfigImpact marks external scaffold toggles as local-only", func(t *testing.T) {
		external := configpkg.Default()
		external.Stack.Managed = false
		external.Setup.ScaffoldDefaultStack = false
		external.ApplyDerivedFields()

		impact := &configImpact{}
		classifyConfigImpact(impact, "setup.scaffold_default_stack", external, external)
		if !impact.localOnly || impact.composeTemplate || impact.manualFollowUp {
			t.Fatalf("unexpected external scaffold impact: %+v", *impact)
		}
	})

	t.Run("selectedFieldEffect returns base text when the follow-up hook is blank", func(t *testing.T) {
		originalSpecific := configEditorSpecificFieldEffect
		originalFollowUp := configEditorEffectFollowUp
		t.Cleanup(func() {
			configEditorSpecificFieldEffect = originalSpecific
			configEditorEffectFollowUp = originalFollowUp
		})

		configEditorSpecificFieldEffect = func(configFieldSpec, configpkg.Config) string { return "base effect" }
		configEditorEffectFollowUp = func(configFieldSpec, configpkg.Config) string { return "" }

		if got := selectedFieldEffect(configFieldSpec{Key: "custom"}, configpkg.Default()); got != "base effect" {
			t.Fatalf("unexpected selected field effect: %q", got)
		}
	})

	t.Run("specificFieldEffect falls back when no package manager recommendation exists", func(t *testing.T) {
		originalCurrentRecommendation := configEditorCurrentPackageManagerRecommendation
		t.Cleanup(func() { configEditorCurrentPackageManagerRecommendation = originalCurrentRecommendation })
		configEditorCurrentPackageManagerRecommendation = func() system.PackageManagerRecommendation {
			return system.PackageManagerRecommendation{}
		}

		spec := testConfigSpecByKey(t, "system.package_manager")
		if got := specificFieldEffect(spec, configpkg.Default()); got != "Controls which package manager setup and doctor fix use for host package installs." {
			t.Fatalf("unexpected package-manager effect: %q", got)
		}
	})

	t.Run("configMarshalConfig surfaces marshal errors and parseImageVersionTag rejects images without tags", func(t *testing.T) {
		originalMarshal := configEditorMarshal
		t.Cleanup(func() { configEditorMarshal = originalMarshal })
		expectedErr := errors.New("marshal failed")
		configEditorMarshal = func(configpkg.Config) ([]byte, error) {
			return nil, expectedErr
		}

		if _, err := configMarshalConfig(configpkg.Default()); !errors.Is(err, expectedErr) {
			t.Fatalf("expected configMarshalConfig to surface %v, got %v", expectedErr, err)
		}
		if major, minor, ok := parseImageVersionTag("redis"); ok || major != 0 || minor != 0 {
			t.Fatalf("expected images without tags to fail parsing, got (%d, %d, %v)", major, minor, ok)
		}
	})
}
