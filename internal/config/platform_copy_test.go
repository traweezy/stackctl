package config

import (
	"strings"
	"testing"

	"github.com/traweezy/stackctl/internal/system"
)

func TestCurrentPlatformCopyHelpersMatchCurrentPlatform(t *testing.T) {
	current := system.CurrentPlatform()

	if got, want := CurrentCockpitHelperDescription(), CockpitHelperDescriptionForPlatform(current); got != want {
		t.Fatalf("expected current cockpit helper copy %q, got %q", want, got)
	}

	if got, want := CurrentCockpitInstallDescription(), CockpitInstallDescriptionForPlatform(current); got != want {
		t.Fatalf("expected current cockpit install copy %q, got %q", want, got)
	}

	if got, want := CurrentPackageManagerFieldDescription(), PackageManagerFieldDescriptionForPlatform(current); got != want {
		t.Fatalf("expected current package-manager copy %q, got %q", want, got)
	}
}

func TestNormalizeCockpitSettingsUsesConfigPlatformFallback(t *testing.T) {
	cfg := Default()
	cfg.System.PackageManager = "brew"
	cfg.Setup.IncludeCockpit = true
	cfg.Setup.InstallCockpit = true

	NormalizeCockpitSettings(&cfg)

	if !cfg.Setup.IncludeCockpit {
		t.Fatalf("expected helper output to stay enabled: %+v", cfg.Setup)
	}
	if cfg.Setup.InstallCockpit {
		t.Fatalf("expected unsupported config platform to clear install_cockpit: %+v", cfg.Setup)
	}
}

func TestCockpitInstallEnableReasonForConfigUsesConfigPlatformFallback(t *testing.T) {
	cfg := Default()
	cfg.System.PackageManager = "brew"

	reason := CockpitInstallEnableReasonForConfig(cfg)
	if !strings.Contains(reason, "cannot install Cockpit") {
		t.Fatalf("expected unsupported config platform reason, got %q", reason)
	}
}

func TestPlatformCopyHelpersCoverNilAndNoRecommendationBranches(t *testing.T) {
	t.Setenv("PATH", "")

	if got := PackageManagerFieldDescriptionForPlatform(system.Platform{}); !strings.Contains(got, "should use for setup and doctor fix flows") {
		t.Fatalf("expected package-manager fallback copy, got %q", got)
	}

	NormalizeCockpitSettings(nil)
	NormalizeCockpitSettingsForPlatform(nil, system.Platform{})
}
