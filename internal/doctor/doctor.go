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
	listContainers     func(context.Context) ([]system.Container, error)
	redisOvercommit    func(context.Context) (system.OvercommitStatus, error)
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
		listContainers: func(ctx context.Context) ([]system.Container, error) {
			return system.ListContainers(ctx, system.CaptureResult)
		},
		redisOvercommit: system.RedisOvercommitStatus,
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
	cockpitEnabled := !cfgLoaded || cfg.CockpitEnabled()

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
		report.add(output.StatusMiss, "podman not installed")
	}

	if deps.podmanComposeAvail(ctx) {
		report.add(output.StatusOK, "podman compose available")
	} else {
		report.add(output.StatusMiss, "podman compose not available")
	}

	if deps.commandExists("buildah") {
		report.add(output.StatusOK, "buildah installed")
	} else {
		report.add(output.StatusMiss, "buildah not installed")
	}

	if deps.commandExists("skopeo") {
		report.add(output.StatusOK, "skopeo installed")
	} else {
		report.add(output.StatusMiss, "skopeo not installed")
	}

	if deps.commandExists("ss") {
		report.add(output.StatusOK, "ss available")
	} else {
		report.add(output.StatusMiss, "ss not available")
	}

	if opener := deps.openCommandName(); opener != "" {
		report.add(output.StatusOK, fmt.Sprintf("%s available", opener))
	} else {
		report.add(output.StatusMiss, "browser opener not available")
	}

	cockpit := deps.cockpitStatus(ctx)
	if cockpitEnabled {
		if cockpit.Installed {
			report.add(output.StatusOK, "cockpit.socket installed")
			if cockpit.Active {
				report.add(output.StatusOK, "cockpit.socket active")
			} else {
				report.add(output.StatusWarn, fmt.Sprintf("cockpit.socket %s", cockpit.State))
			}
		} else {
			report.add(output.StatusMiss, "cockpit.socket not installed")
		}
	}

	if cfgLoaded {
		containers := []system.Container(nil)
		podmanAvailable := deps.commandExists("podman")
		if podmanAvailable {
			loadedContainers, err := deps.listContainers(ctx)
			if err != nil {
				report.add(output.StatusFail, fmt.Sprintf("container inspection failed: %v", err))
			} else {
				containers = system.FilterContainersByName(loadedContainers, configuredContainerNames(cfg))
			}
		}

		containerByName := make(map[string]system.Container, len(containers))
		for _, container := range containers {
			for _, name := range container.Names {
				containerByName[name] = container
			}
		}

		for _, service := range configuredServices(cfg) {
			inUse, err := deps.portInUse(service.Port)
			if err != nil {
				report.add(output.StatusFail, fmt.Sprintf("port %d check failed: %v", service.Port, err))
				continue
			}

			container, ok := containerByName[service.ContainerName]
			switch {
			case ok && containerBindsHostPort(container, service.Port):
				report.add(output.StatusOK, fmt.Sprintf("port %d is mapped by %s container", service.Port, service.Name))
			case inUse:
				report.add(output.StatusWarn, fmt.Sprintf("port %d is in use by another process or container, not %s", service.Port, service.Name))
			default:
				report.add(output.StatusOK, fmt.Sprintf("port %d is free for %s", service.Port, service.Name))
			}

			if !podmanAvailable {
				continue
			}
			if !ok {
				report.add(output.StatusWarn, fmt.Sprintf("%s container not found", service.Name))
				continue
			}
			if container.State == "running" {
				report.add(output.StatusOK, fmt.Sprintf("%s container running", service.Name))
				continue
			}

			report.add(output.StatusWarn, fmt.Sprintf("%s container not running (%s)", service.Name, container.Status))
		}

		if cfg.CockpitEnabled() {
			cockpitInUse, err := deps.portInUse(cfg.Ports.Cockpit)
			if err != nil {
				report.add(output.StatusFail, fmt.Sprintf("port %d check failed: %v", cfg.Ports.Cockpit, err))
			} else if cockpitInUse && cockpit.Active {
				report.add(output.StatusOK, fmt.Sprintf("port %d is in use by cockpit", cfg.Ports.Cockpit))
			} else if cockpit.Active {
				report.add(output.StatusWarn, fmt.Sprintf("cockpit.socket active but port %d is not listening", cfg.Ports.Cockpit))
			} else if cockpitInUse {
				report.add(output.StatusWarn, fmt.Sprintf("port %d is in use by another process, not cockpit", cfg.Ports.Cockpit))
			} else {
				report.add(output.StatusOK, fmt.Sprintf("port %d is free for cockpit", cfg.Ports.Cockpit))
			}
		}

		overcommit, err := deps.redisOvercommit(ctx)
		if err == nil && overcommit.Supported {
			if overcommit.Value == 1 {
				report.add(output.StatusOK, "vm.overcommit_memory is set to 1 for Redis")
			} else {
				report.add(output.StatusWarn, "set vm.overcommit_memory=1 to avoid Redis memory overcommit warnings")
			}
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

type configuredService struct {
	Name          string
	ContainerName string
	Port          int
}

func configuredServices(cfg configpkg.Config) []configuredService {
	services := make([]configuredService, 0, 4)
	if cfg.PostgresEnabled() {
		services = append(services, configuredService{
			Name:          "postgres",
			ContainerName: cfg.Services.PostgresContainer,
			Port:          cfg.Ports.Postgres,
		})
	}
	if cfg.RedisEnabled() {
		services = append(services, configuredService{
			Name:          "redis",
			ContainerName: cfg.Services.RedisContainer,
			Port:          cfg.Ports.Redis,
		})
	}
	if cfg.NATSEnabled() {
		services = append(services, configuredService{
			Name:          "nats",
			ContainerName: cfg.Services.NATSContainer,
			Port:          cfg.Ports.NATS,
		})
	}
	if cfg.PgAdminEnabled() {
		services = append(services, configuredService{
			Name:          "pgadmin",
			ContainerName: cfg.Services.PgAdminContainer,
			Port:          cfg.Ports.PgAdmin,
		})
	}

	return services
}

func configuredContainerNames(cfg configpkg.Config) []string {
	names := make([]string, 0, len(configuredServices(cfg)))
	for _, service := range configuredServices(cfg) {
		names = append(names, service.ContainerName)
	}
	return names
}

func containerBindsHostPort(container system.Container, port int) bool {
	for _, mapping := range container.Ports {
		if mapping.HostPort == port {
			return true
		}
	}

	return false
}
