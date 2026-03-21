package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

type Check struct {
	Status  string
	Message string
}

type Report struct {
	Checks    []Check
	OKCount   int
	WarnCount int
	MissCount int
	FailCount int
}

type dependencies struct {
	configFilePath     func() (string, error)
	loadConfig         func(string) (configpkg.Config, error)
	validateConfig     func(configpkg.Config) []configpkg.ValidationIssue
	composePath        func(configpkg.Config) string
	stat               func(string) (os.FileInfo, error)
	commandExists      func(string) bool
	podmanComposeAvail func(context.Context) bool
	openCommandName    func() string
	cockpitStatus      func(context.Context) system.CockpitState
	portInUse          func(int) (bool, error)
	anyContainerExists func(context.Context, []string) (bool, error)
}

func Run(ctx context.Context) (Report, error) {
	return runWithDeps(ctx, defaultDependencies())
}

func defaultDependencies() dependencies {
	return dependencies{
		configFilePath:     configpkg.ConfigFilePath,
		loadConfig:         configpkg.Load,
		validateConfig:     configpkg.Validate,
		composePath:        configpkg.ComposePath,
		stat:               os.Stat,
		commandExists:      system.CommandExists,
		podmanComposeAvail: system.PodmanComposeAvailable,
		openCommandName:    system.OpenCommandName,
		cockpitStatus:      system.CockpitStatus,
		portInUse:          system.PortInUse,
		anyContainerExists: system.AnyContainerExists,
	}
}

func runWithDeps(ctx context.Context, deps dependencies) (Report, error) {
	report := Report{Checks: make([]Check, 0, 16)}

	path, err := deps.configFilePath()
	if err != nil {
		return report, err
	}

	cfg, cfgLoaded, err := configResult(deps, path)
	if err != nil {
		return report, err
	}

	if cfgLoaded {
		report.add(output.StatusOK, fmt.Sprintf("config file found: %s", path))

		issues := deps.validateConfig(cfg)
		if len(issues) == 0 {
			report.add(output.StatusOK, "config is valid")
		} else {
			report.add(output.StatusFail, fmt.Sprintf("config invalid (%d issue(s))", len(issues)))
		}

		if info, err := deps.stat(cfg.Stack.Dir); err == nil && info.IsDir() {
			report.add(output.StatusOK, fmt.Sprintf("stack directory exists: %s", cfg.Stack.Dir))
		} else {
			report.add(output.StatusFail, fmt.Sprintf("stack directory missing: %s", cfg.Stack.Dir))
		}

		composePath := deps.composePath(cfg)
		if info, err := deps.stat(composePath); err == nil && !info.IsDir() {
			report.add(output.StatusOK, fmt.Sprintf("compose file found: %s", composePath))
		} else {
			report.add(output.StatusFail, fmt.Sprintf("compose file missing: %s", composePath))
		}
	} else {
		report.add(output.StatusMiss, fmt.Sprintf("config file not found: %s", path))
	}

	if deps.commandExists("podman") {
		report.add(output.StatusOK, "podman installed")
	} else {
		report.add(output.StatusMiss, "podman installed")
	}

	if deps.podmanComposeAvail(ctx) {
		report.add(output.StatusOK, "podman compose available")
	} else {
		report.add(output.StatusMiss, "podman compose available")
	}

	if deps.commandExists("buildah") {
		report.add(output.StatusOK, "buildah installed")
	} else {
		report.add(output.StatusMiss, "buildah installed")
	}

	if deps.commandExists("skopeo") {
		report.add(output.StatusOK, "skopeo installed")
	} else {
		report.add(output.StatusMiss, "skopeo installed")
	}

	if deps.commandExists("ss") {
		report.add(output.StatusOK, "ss available")
	} else {
		report.add(output.StatusMiss, "ss available")
	}

	if opener := deps.openCommandName(); opener != "" {
		report.add(output.StatusOK, fmt.Sprintf("%s available", opener))
	} else {
		report.add(output.StatusMiss, "browser opener available")
	}

	cockpit := deps.cockpitStatus(ctx)
	if cockpit.Installed {
		report.add(output.StatusOK, "cockpit.socket installed")
		if cockpit.Active {
			report.add(output.StatusOK, "cockpit.socket active")
		} else {
			report.add(output.StatusWarn, fmt.Sprintf("cockpit.socket %s", cockpit.State))
		}
	} else {
		report.add(output.StatusMiss, "cockpit.socket installed")
	}

	if cfgLoaded {
		for _, portCheck := range []struct {
			name string
			port int
		}{
			{name: "postgres", port: cfg.Ports.Postgres},
			{name: "redis", port: cfg.Ports.Redis},
			{name: "pgadmin", port: cfg.Ports.PgAdmin},
			{name: "cockpit", port: cfg.Ports.Cockpit},
		} {
			inUse, err := deps.portInUse(portCheck.port)
			if err != nil {
				report.add(output.StatusFail, fmt.Sprintf("port %d check failed: %v", portCheck.port, err))
				continue
			}
			if inUse {
				report.add(output.StatusWarn, fmt.Sprintf("port %d already in use for %s", portCheck.port, portCheck.name))
			} else {
				report.add(output.StatusOK, fmt.Sprintf("port %d is free for %s", portCheck.port, portCheck.name))
			}
		}

		containerNames := []string{
			cfg.Services.PostgresContainer,
			cfg.Services.RedisContainer,
		}
		if cfg.Setup.IncludePgAdmin {
			containerNames = append(containerNames, cfg.Services.PgAdminContainer)
		}

		exists, err := deps.anyContainerExists(ctx, containerNames)
		if err != nil {
			report.add(output.StatusFail, fmt.Sprintf("container inspection failed: %v", err))
		} else if exists {
			report.add(output.StatusOK, "running containers from this stack exist")
		} else {
			report.add(output.StatusWarn, "no containers from this stack were found")
		}
	}

	return report, nil
}

func (r Report) HasFailures() bool {
	return r.FailCount > 0 || r.MissCount > 0
}

func CheckPassed(report Report, message string) bool {
	for _, check := range report.Checks {
		if check.Message == message {
			return check.Status == output.StatusOK
		}
	}

	return false
}

func (r *Report) add(status, message string) {
	r.Checks = append(r.Checks, Check{Status: status, Message: message})

	switch status {
	case output.StatusOK:
		r.OKCount++
	case output.StatusWarn:
		r.WarnCount++
	case output.StatusMiss:
		r.MissCount++
	case output.StatusFail:
		r.FailCount++
	}
}

func configResult(deps dependencies, path string) (configpkg.Config, bool, error) {
	cfg, err := deps.loadConfig(path)
	if err == nil {
		return cfg, true, nil
	}
	if errors.Is(err, configpkg.ErrNotFound) {
		return configpkg.Config{}, false, nil
	}
	if strings.TrimSpace(err.Error()) == "" {
		return configpkg.Config{}, false, nil
	}

	return configpkg.Config{}, false, err
}
