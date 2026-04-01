package system

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

type Requirement string

const (
	RequirementPodman          Requirement = "podman"
	RequirementComposeProvider Requirement = "podman compose provider"
	RequirementBuildah         Requirement = "buildah"
	RequirementSkopeo          Requirement = "skopeo"
	RequirementCockpit         Requirement = "cockpit"
)

type InstallPlan struct {
	Packages    []string
	Unsupported []Requirement
}

type packageBackend struct {
	command  string
	packages map[Requirement][]string
	install  func(context.Context, Runner, []string) error
}

const (
	zypperInstallAttempts = 3
	zypperRetryDelay      = 2 * time.Second
)

var packageBackends = map[string]packageBackend{
	"apt": {
		command: "apt-get",
		packages: map[Requirement][]string{
			RequirementPodman:          {"podman"},
			RequirementComposeProvider: {"podman-compose"},
			RequirementBuildah:         {"buildah"},
			RequirementSkopeo:          {"skopeo"},
		},
		install: func(ctx context.Context, runner Runner, packages []string) error {
			if err := runner.Run(ctx, "", "sudo", "apt-get", "update"); err != nil {
				return err
			}
			args := append([]string{"apt-get", "install", "-y"}, packages...)
			return runner.Run(ctx, "", "sudo", args...)
		},
	},
	"dnf": {
		command: "dnf",
		packages: map[Requirement][]string{
			RequirementPodman:          {"podman"},
			RequirementComposeProvider: {"podman-compose"},
			RequirementBuildah:         {"buildah"},
			RequirementSkopeo:          {"skopeo"},
			RequirementCockpit:         {"cockpit", "cockpit-podman"},
		},
		install: func(ctx context.Context, runner Runner, packages []string) error {
			args := append([]string{"dnf", "install", "-y"}, packages...)
			return runner.Run(ctx, "", "sudo", args...)
		},
	},
	"yum": {
		command: "yum",
		packages: map[Requirement][]string{
			RequirementPodman:          {"podman"},
			RequirementComposeProvider: {"podman-compose"},
			RequirementBuildah:         {"buildah"},
			RequirementSkopeo:          {"skopeo"},
			RequirementCockpit:         {"cockpit", "cockpit-podman"},
		},
		install: func(ctx context.Context, runner Runner, packages []string) error {
			args := append([]string{"yum", "install", "-y"}, packages...)
			return runner.Run(ctx, "", "sudo", args...)
		},
	},
	"pacman": {
		command: "pacman",
		packages: map[Requirement][]string{
			RequirementPodman:          {"podman"},
			RequirementComposeProvider: {"podman-compose"},
			RequirementBuildah:         {"buildah"},
			RequirementSkopeo:          {"skopeo"},
			RequirementCockpit:         {"cockpit", "cockpit-podman"},
		},
		install: func(ctx context.Context, runner Runner, packages []string) error {
			args := append([]string{"pacman", "-Syu", "--noconfirm", "--needed"}, packages...)
			return runner.Run(ctx, "", "sudo", args...)
		},
	},
	"zypper": {
		command: "zypper",
		packages: map[Requirement][]string{
			RequirementPodman:          {"podman"},
			RequirementComposeProvider: {"podman-compose"},
			RequirementBuildah:         {"buildah"},
			RequirementSkopeo:          {"skopeo"},
			RequirementCockpit:         {"cockpit", "cockpit-podman"},
		},
		install: func(ctx context.Context, runner Runner, packages []string) error {
			return runZypperInstallWithRetry(ctx, runner, packages)
		},
	},
	"apk": {
		command: "apk",
		packages: map[Requirement][]string{
			RequirementPodman:          {"podman"},
			RequirementComposeProvider: {"podman-compose"},
			RequirementBuildah:         {"buildah"},
			RequirementSkopeo:          {"skopeo"},
		},
		install: func(ctx context.Context, runner Runner, packages []string) error {
			args := append([]string{"apk", "add"}, packages...)
			return runner.Run(ctx, "", "sudo", args...)
		},
	},
	"brew": {
		command: "brew",
		packages: map[Requirement][]string{
			RequirementPodman:          {"podman"},
			RequirementComposeProvider: {"podman-compose"},
			RequirementSkopeo:          {"skopeo"},
		},
		install: func(ctx context.Context, runner Runner, packages []string) error {
			args := append([]string{"install"}, packages...)
			return runner.Run(ctx, "", "brew", args...)
		},
	},
}

func ResolveInstallPlan(packageManager string, requirements []Requirement) (InstallPlan, error) {
	normalized := normalizePackageManager(packageManager)
	backend, ok := packageBackends[normalized]
	if !ok {
		return InstallPlan{}, fmt.Errorf("unsupported package manager %q", packageManager)
	}

	plan := InstallPlan{
		Packages:    make([]string, 0, len(requirements)*2),
		Unsupported: make([]Requirement, 0, len(requirements)),
	}
	seenPackages := make(map[string]struct{}, len(requirements)*2)
	seenUnsupported := make(map[Requirement]struct{}, len(requirements))

	for _, requirement := range requirements {
		packages, supported := backend.packages[requirement]
		if !supported {
			if _, exists := seenUnsupported[requirement]; !exists {
				seenUnsupported[requirement] = struct{}{}
				plan.Unsupported = append(plan.Unsupported, requirement)
			}
			continue
		}

		for _, pkg := range packages {
			if _, exists := seenPackages[pkg]; exists {
				continue
			}
			seenPackages[pkg] = struct{}{}
			plan.Packages = append(plan.Packages, pkg)
		}
	}

	return plan, nil
}

func runZypperInstallWithRetry(ctx context.Context, runner Runner, packages []string) error {
	refreshArgs := []string{"zypper", "--non-interactive", "--gpg-auto-import-keys", "refresh", "--force"}
	cleanArgs := []string{"zypper", "--non-interactive", "clean", "--all"}
	installArgs := append([]string{"zypper", "--non-interactive", "install"}, packages...)

	var lastErr error
	for attempt := 1; attempt <= zypperInstallAttempts; attempt++ {
		writeRunnerNotice(runner.Stdout, "zypper install attempt %d/%d\n", attempt, zypperInstallAttempts)
		if err := runner.Run(ctx, "", "sudo", cleanArgs...); err != nil {
			lastErr = err
		} else if err := runner.Run(ctx, "", "sudo", refreshArgs...); err != nil {
			lastErr = err
		} else if err := runner.Run(ctx, "", "sudo", installArgs...); err != nil {
			lastErr = err
		} else {
			return nil
		}

		if attempt == zypperInstallAttempts {
			break
		}

		writeRunnerNotice(runner.Stderr, "zypper install attempt %d failed; cleaning metadata and retrying\n", attempt)
		timer := time.NewTimer(zypperRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	return fmt.Errorf("zypper install failed after %d attempts: %w", zypperInstallAttempts, lastErr)
}

func writeRunnerNotice(writer io.Writer, format string, args ...any) {
	if writer == nil {
		return
	}
	_, _ = fmt.Fprintf(writer, format, args...)
}

func InstallPackages(ctx context.Context, runner Runner, packageManager string, requirements []Requirement) ([]string, error) {
	if len(requirements) == 0 {
		return nil, nil
	}

	normalized := normalizePackageManager(packageManager)
	backend, ok := packageBackends[normalized]
	if !ok {
		return nil, fmt.Errorf("unsupported package manager %q", packageManager)
	}
	if strings.TrimSpace(backend.command) == "" || !CommandExists(backend.command) {
		return nil, fmt.Errorf(
			"package manager %q is configured but the %s command is not installed on this machine",
			normalized,
			backend.command,
		)
	}

	plan, err := ResolveInstallPlan(normalized, requirements)
	if err != nil {
		return nil, err
	}
	if len(plan.Unsupported) > 0 {
		names := make([]string, 0, len(plan.Unsupported))
		for _, requirement := range plan.Unsupported {
			names = append(names, string(requirement))
		}
		return nil, fmt.Errorf(
			"package manager %q does not support automatic installation for: %s",
			normalized,
			strings.Join(names, ", "),
		)
	}
	if len(plan.Packages) == 0 {
		return nil, nil
	}
	if err := backend.install(ctx, runner, plan.Packages); err != nil {
		return nil, err
	}

	return plan.Packages, nil
}
