package config

import (
	"fmt"
	"strings"

	"github.com/traweezy/stackctl/internal/system"
)

func CurrentCockpitHelperDescription() string {
	return CockpitHelperDescriptionForPlatform(system.CurrentPlatform())
}

func CockpitHelperDescriptionForConfig(cfg Config) string {
	return CockpitHelperDescriptionForPlatform(platformForInteractiveConfig(cfg, system.CurrentPlatform()))
}

func CockpitHelperDescriptionForPlatform(platform system.Platform) string {
	if platform.SupportsCockpit() {
		return "Enable Cockpit in stackctl helper output, dashboard actions, and open commands. Cockpit is managed outside the compose stack on this host."
	}
	if platform.GOOS == "darwin" {
		return "Enable Cockpit in stackctl helper output and open commands only. stackctl cannot install or manage Cockpit automatically on macOS."
	}
	return "Enable Cockpit in stackctl helper output and open commands only. stackctl cannot install or manage Cockpit automatically on this host."
}

func CurrentCockpitInstallDescription() string {
	return CockpitInstallDescriptionForPlatform(system.CurrentPlatform())
}

func CockpitInstallDescriptionForConfig(cfg Config) string {
	return CockpitInstallDescriptionForPlatform(platformForInteractiveConfig(cfg, system.CurrentPlatform()))
}

func CockpitInstallDescriptionForPlatform(platform system.Platform) string {
	switch {
	case platform.SupportsCockpitAutoInstall():
		return "If enabled, stackctl setup and doctor fix can install and enable Cockpit automatically on this host."
	case platform.SupportsCockpit():
		return "If enabled, stackctl will keep Cockpit guidance in setup and doctor, but installation still needs to be handled manually on this host."
	case platform.GOOS == "darwin":
		return "This host does not support Cockpit installation in stackctl. Leave this off unless you plan to manage Cockpit manually outside the tool."
	default:
		return "This host does not support Cockpit installation in stackctl. Leave this off unless you plan to manage Cockpit manually outside the tool."
	}
}

func CurrentPackageManagerFieldDescription() string {
	return PackageManagerFieldDescriptionForPlatform(system.CurrentPlatform())
}

func PackageManagerFieldDescriptionForConfig(cfg Config) string {
	return PackageManagerFieldDescriptionForPlatform(platformForInteractiveConfig(cfg, system.CurrentPlatform()))
}

func PackageManagerFieldDescriptionForPlatform(platform system.Platform) string {
	recommendation := system.RecommendPackageManager(platform, system.CommandExists)
	if recommendation.Name == "" {
		return "The package manager stackctl should use for setup and doctor fix flows on this host."
	}
	return fmt.Sprintf(
		"The package manager stackctl should use for setup and doctor fix flows on this host. %s",
		system.FormatPackageManagerRecommendation(recommendation),
	)
}

func platformForInteractiveConfig(cfg Config, fallback system.Platform) system.Platform {
	platform := fallback
	if packageManager := strings.ToLower(strings.TrimSpace(cfg.System.PackageManager)); packageManager != "" {
		platform.PackageManager = packageManager
	}
	return platform
}

func NormalizeCockpitSettings(cfg *Config) {
	if cfg == nil {
		return
	}
	NormalizeCockpitSettingsForPlatform(cfg, platformForInteractiveConfig(*cfg, system.CurrentPlatform()))
}

func NormalizeCockpitSettingsForPlatform(cfg *Config, platform system.Platform) {
	if cfg == nil {
		return
	}
	if !cfg.Setup.IncludeCockpit || !platform.SupportsCockpit() {
		cfg.Setup.InstallCockpit = false
	}
}

func CockpitInstallEnableReasonForConfig(cfg Config) string {
	return CockpitInstallEnableReasonForPlatform(platformForInteractiveConfig(cfg, system.CurrentPlatform()))
}

func CockpitInstallEnableReasonForPlatform(platform system.Platform) string {
	if platform.SupportsCockpit() {
		return ""
	}
	return "This host cannot install Cockpit through stackctl. Keep helpers only, or manage Cockpit separately."
}
