package system

import (
	"fmt"
	"strings"
)

type PackageManagerRecommendation struct {
	Name      string
	Command   string
	Available bool
	Reason    string
}

type PackageManagerChoice struct {
	Name    string
	Command string
	Notice  string
}

func CurrentPackageManagerRecommendation() PackageManagerRecommendation {
	return RecommendPackageManager(CurrentPlatform(), CommandExists)
}

func RecommendPackageManager(platform Platform, commandExists func(string) bool) PackageManagerRecommendation {
	normalized := normalizePackageManager(platform.PackageManager)
	if normalized != "" && PackageManagerSupported(normalized) {
		command := PackageManagerCommand(normalized)
		return PackageManagerRecommendation{
			Name:      normalized,
			Command:   command,
			Available: command != "" && commandExists(command),
			Reason:    platformPackageManagerReason(platform),
		}
	}

	for _, candidate := range []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "brew"} {
		command := PackageManagerCommand(candidate)
		if command == "" || !commandExists(command) {
			continue
		}
		return PackageManagerRecommendation{
			Name:      candidate,
			Command:   command,
			Available: true,
			Reason:    "detected from the available package-manager commands on this machine",
		}
	}

	return PackageManagerRecommendation{}
}

func FormatPackageManagerRecommendation(rec PackageManagerRecommendation) string {
	if rec.Name == "" {
		return "No supported package manager was detected on this machine."
	}
	if rec.Available {
		return fmt.Sprintf(
			"Recommended for this machine: %s (%s; available via %s).",
			rec.Name,
			rec.Reason,
			rec.Command,
		)
	}
	return fmt.Sprintf(
		"Recommended for this machine: %s (%s; install %s first).",
		rec.Name,
		rec.Reason,
		rec.Command,
	)
}

func ResolvePackageManagerChoice(configured string, platform Platform, commandExists func(string) bool) (PackageManagerChoice, error) {
	recommendation := RecommendPackageManager(platform, commandExists)
	normalized := normalizePackageManager(configured)

	if normalized == "" {
		if recommendation.Name == "" {
			return PackageManagerChoice{}, fmt.Errorf("no supported package manager was detected on this machine")
		}
		if !recommendation.Available {
			return PackageManagerChoice{}, fmt.Errorf(
				"no package manager is configured; %s is recommended on this machine, but %s is not installed",
				recommendation.Name,
				recommendation.Command,
			)
		}
		return PackageManagerChoice{
			Name:    recommendation.Name,
			Command: recommendation.Command,
			Notice: fmt.Sprintf(
				"no package manager is configured; using detected %s for this run. Update system.package_manager to keep it.",
				recommendation.Name,
			),
		}, nil
	}

	if !PackageManagerSupported(normalized) {
		if recommendation.Name != "" && recommendation.Available {
			return PackageManagerChoice{
				Name:    recommendation.Name,
				Command: recommendation.Command,
				Notice: fmt.Sprintf(
					"configured package manager %q is unsupported; using detected %s for this run. Update system.package_manager to keep it.",
					normalized,
					recommendation.Name,
				),
			}, nil
		}
		return PackageManagerChoice{}, fmt.Errorf("unsupported package manager %q", normalized)
	}

	command := PackageManagerCommand(normalized)
	if command != "" && commandExists(command) {
		return PackageManagerChoice{Name: normalized, Command: command}, nil
	}

	if recommendation.Name != "" && recommendation.Available && recommendation.Name != normalized {
		return PackageManagerChoice{
			Name:    recommendation.Name,
			Command: recommendation.Command,
			Notice: fmt.Sprintf(
				"configured package manager %q is not installed; using detected %s for this run. Update system.package_manager to keep it.",
				normalized,
				recommendation.Name,
			),
		}, nil
	}

	if recommendation.Name == normalized && recommendation.Command != "" {
		return PackageManagerChoice{}, fmt.Errorf(
			"package manager %q is recommended for this machine, but %s is not installed",
			normalized,
			recommendation.Command,
		)
	}

	return PackageManagerChoice{}, fmt.Errorf(
		"configured package manager %q is not installed; install %s or update system.package_manager",
		normalized,
		command,
	)
}

func PackageManagerSupported(packageManager string) bool {
	_, ok := packageBackends[normalizePackageManager(packageManager)]
	return ok
}

func PackageManagerCommand(packageManager string) string {
	backend, ok := packageBackends[normalizePackageManager(packageManager)]
	if !ok {
		return ""
	}
	return backend.command
}

func platformPackageManagerReason(platform Platform) string {
	switch {
	case platform.GOOS == "darwin":
		return "recommended for this macOS host"
	case platform.matchesDistro("ubuntu", "debian", "linuxmint", "pop", "neon", "elementary", "zorin", "tuxedo"):
		return "recommended for this Debian/Ubuntu-family Linux host"
	case platform.matchesDistro("fedora", "centos", "rhel", "rocky", "almalinux", "ol", "amzn"):
		return "recommended for this Fedora/RHEL-family Linux host"
	case platform.matchesDistro("arch", "manjaro", "endeavouros"):
		return "recommended for this Arch-family Linux host"
	case platform.matchesDistro("opensuse", "opensuse-leap", "opensuse-tumbleweed", "sles", "sle_hpc"):
		return "recommended for this openSUSE-family Linux host"
	case platform.matchesDistro("alpine"):
		return "recommended for this Alpine Linux host"
	case strings.TrimSpace(platform.DistroID) != "":
		return fmt.Sprintf("recommended for this %s Linux host", strings.TrimSpace(platform.DistroID))
	default:
		return "detected from the current host"
	}
}
