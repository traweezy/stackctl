package cmd

import (
	"context"
	"errors"
	"fmt"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

type serviceKind string

const (
	serviceKindStack    serviceKind = "stack"
	serviceKindHostTool serviceKind = "host-tool"
)

type serviceCopySpec struct {
	PrimaryAlias string
	Aliases      []string
	Label        string
	Resolve      func(configpkg.Config) (string, error)
}

type serviceDefinition struct {
	Key                 string
	DisplayName         string
	Icon                string
	Aliases             []string
	Kind                serviceKind
	Enabled             func(configpkg.Config) bool
	ContainerName       func(configpkg.Config) string
	PrimaryPort         func(configpkg.Config) int
	PrimaryPortLabel    string
	DefaultInternalPort int
	WaitOnStart         bool
	BuildRuntime        func(context.Context, configpkg.Config, map[string]system.Container) runtimeService
	ConnectionEntries   func(configpkg.Config) []connectionEntry
	CopyTargets         func() []serviceCopySpec
}

func serviceDefinitions() []serviceDefinition {
	return []serviceDefinition{
		{
			Key:                 "postgres",
			DisplayName:         "Postgres",
			Icon:                "🗄️",
			Aliases:             []string{"postgres", "pg"},
			Kind:                serviceKindStack,
			Enabled:             func(cfg configpkg.Config) bool { return cfg.PostgresEnabled() },
			ContainerName:       func(cfg configpkg.Config) string { return cfg.Services.PostgresContainer },
			PrimaryPort:         func(cfg configpkg.Config) int { return cfg.Ports.Postgres },
			PrimaryPortLabel:    "postgres port listening",
			DefaultInternalPort: 5432,
			WaitOnStart:         true,
			BuildRuntime: func(_ context.Context, cfg configpkg.Config, containerByName map[string]system.Container) runtimeService {
				return runtimeService{
					Name:          "postgres",
					Icon:          "🗄️",
					DisplayName:   "Postgres",
					Status:        containerStatus(containerByName, cfg.Services.PostgresContainer),
					ContainerName: cfg.Services.PostgresContainer,
					Image:         cfg.Services.Postgres.Image,
					DataVolume:    cfg.Services.Postgres.DataVolume,
					Host:          cfg.Connection.Host,
					ExternalPort:  cfg.Ports.Postgres,
					InternalPort:  containerInternalPort(containerByName, cfg.Services.PostgresContainer, cfg.Ports.Postgres),
					Database:      cfg.Connection.PostgresDatabase,
					MaintenanceDB: cfg.Services.Postgres.MaintenanceDatabase,
					Username:      cfg.Connection.PostgresUsername,
					Password:      cfg.Connection.PostgresPassword,
					DSN:           postgresDSN(cfg),
				}
			},
			ConnectionEntries: func(cfg configpkg.Config) []connectionEntry {
				return []connectionEntry{{Name: "Postgres", Value: postgresDSN(cfg)}}
			},
			CopyTargets: func() []serviceCopySpec {
				return []serviceCopySpec{
					{
						PrimaryAlias: "postgres",
						Aliases:      []string{"postgresdsn"},
						Label:        "postgres DSN",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.PostgresEnabled() {
								return "", errors.New("postgres is not enabled in this stack")
							}
							return postgresDSN(cfg), nil
						},
					},
					{
						PrimaryAlias: "postgres-user",
						Aliases:      []string{"postgresuser", "postgresusername"},
						Label:        "Postgres username",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.PostgresEnabled() {
								return "", errors.New("postgres is not enabled in this stack")
							}
							return cfg.Connection.PostgresUsername, nil
						},
					},
					{
						PrimaryAlias: "postgres-password",
						Aliases:      []string{"postgrespassword"},
						Label:        "Postgres password",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.PostgresEnabled() {
								return "", errors.New("postgres is not enabled in this stack")
							}
							return cfg.Connection.PostgresPassword, nil
						},
					},
					{
						PrimaryAlias: "postgres-database",
						Aliases:      []string{"postgresdatabase", "postgresdb"},
						Label:        "Postgres database",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.PostgresEnabled() {
								return "", errors.New("postgres is not enabled in this stack")
							}
							return cfg.Connection.PostgresDatabase, nil
						},
					},
				}
			},
		},
		{
			Key:                 "redis",
			DisplayName:         "Redis",
			Icon:                "⚡",
			Aliases:             []string{"redis", "rd"},
			Kind:                serviceKindStack,
			Enabled:             func(cfg configpkg.Config) bool { return cfg.RedisEnabled() },
			ContainerName:       func(cfg configpkg.Config) string { return cfg.Services.RedisContainer },
			PrimaryPort:         func(cfg configpkg.Config) int { return cfg.Ports.Redis },
			PrimaryPortLabel:    "redis port listening",
			DefaultInternalPort: 6379,
			WaitOnStart:         true,
			BuildRuntime: func(_ context.Context, cfg configpkg.Config, containerByName map[string]system.Container) runtimeService {
				return runtimeService{
					Name:            "redis",
					Icon:            "⚡",
					DisplayName:     "Redis",
					Status:          containerStatus(containerByName, cfg.Services.RedisContainer),
					ContainerName:   cfg.Services.RedisContainer,
					Image:           cfg.Services.Redis.Image,
					DataVolume:      cfg.Services.Redis.DataVolume,
					Host:            cfg.Connection.Host,
					ExternalPort:    cfg.Ports.Redis,
					InternalPort:    containerInternalPort(containerByName, cfg.Services.RedisContainer, cfg.Ports.Redis),
					Password:        cfg.Connection.RedisPassword,
					AppendOnly:      boolPointer(cfg.Services.Redis.AppendOnly),
					SavePolicy:      cfg.Services.Redis.SavePolicy,
					MaxMemoryPolicy: cfg.Services.Redis.MaxMemoryPolicy,
					DSN:             redisDSN(cfg),
				}
			},
			ConnectionEntries: func(cfg configpkg.Config) []connectionEntry {
				return []connectionEntry{{Name: "Redis", Value: redisDSN(cfg)}}
			},
			CopyTargets: func() []serviceCopySpec {
				return []serviceCopySpec{
					{
						PrimaryAlias: "redis",
						Aliases:      []string{"redisdsn"},
						Label:        "redis DSN",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.RedisEnabled() {
								return "", errors.New("redis is not enabled in this stack")
							}
							return redisDSN(cfg), nil
						},
					},
					{
						PrimaryAlias: "redis-password",
						Aliases:      []string{"redispassword"},
						Label:        "Redis password",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.RedisEnabled() {
								return "", errors.New("redis is not enabled in this stack")
							}
							return cfg.Connection.RedisPassword, nil
						},
					},
				}
			},
		},
		{
			Key:                 "nats",
			DisplayName:         "NATS",
			Icon:                "📡",
			Aliases:             []string{"nats"},
			Kind:                serviceKindStack,
			Enabled:             func(cfg configpkg.Config) bool { return cfg.NATSEnabled() },
			ContainerName:       func(cfg configpkg.Config) string { return cfg.Services.NATSContainer },
			PrimaryPort:         func(cfg configpkg.Config) int { return cfg.Ports.NATS },
			PrimaryPortLabel:    "nats port listening",
			DefaultInternalPort: 4222,
			WaitOnStart:         true,
			BuildRuntime: func(_ context.Context, cfg configpkg.Config, containerByName map[string]system.Container) runtimeService {
				return runtimeService{
					Name:          "nats",
					Icon:          "📡",
					DisplayName:   "NATS",
					Status:        containerStatus(containerByName, cfg.Services.NATSContainer),
					ContainerName: cfg.Services.NATSContainer,
					Image:         cfg.Services.NATS.Image,
					Host:          cfg.Connection.Host,
					ExternalPort:  cfg.Ports.NATS,
					InternalPort:  containerInternalPort(containerByName, cfg.Services.NATSContainer, cfg.Ports.NATS),
					Token:         cfg.Connection.NATSToken,
					DSN:           natsDSN(cfg),
				}
			},
			ConnectionEntries: func(cfg configpkg.Config) []connectionEntry {
				if !cfg.NATSEnabled() {
					return nil
				}
				return []connectionEntry{{Name: "NATS", Value: natsDSN(cfg)}}
			},
			CopyTargets: func() []serviceCopySpec {
				return []serviceCopySpec{
					{
						PrimaryAlias: "nats",
						Aliases:      []string{"natsdsn"},
						Label:        "NATS DSN",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.NATSEnabled() {
								return "", errors.New("nats is not enabled in this stack")
							}
							return natsDSN(cfg), nil
						},
					},
					{
						PrimaryAlias: "nats-token",
						Aliases:      []string{"natstoken"},
						Label:        "NATS token",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.NATSEnabled() {
								return "", errors.New("nats is not enabled in this stack")
							}
							return cfg.Connection.NATSToken, nil
						},
					},
				}
			},
		},
		{
			Key:                 "pgadmin",
			DisplayName:         "pgAdmin",
			Icon:                "🌐",
			Aliases:             []string{"pgadmin"},
			Kind:                serviceKindStack,
			Enabled:             func(cfg configpkg.Config) bool { return cfg.PgAdminEnabled() },
			ContainerName:       func(cfg configpkg.Config) string { return cfg.Services.PgAdminContainer },
			PrimaryPort:         func(cfg configpkg.Config) int { return cfg.Ports.PgAdmin },
			PrimaryPortLabel:    "pgadmin port listening",
			DefaultInternalPort: 80,
			WaitOnStart:         false,
			BuildRuntime: func(_ context.Context, cfg configpkg.Config, containerByName map[string]system.Container) runtimeService {
				return runtimeService{
					Name:          "pgadmin",
					Icon:          "🌐",
					DisplayName:   "pgAdmin",
					Status:        containerStatus(containerByName, cfg.Services.PgAdminContainer),
					ContainerName: cfg.Services.PgAdminContainer,
					Image:         cfg.Services.PgAdmin.Image,
					DataVolume:    cfg.Services.PgAdmin.DataVolume,
					Host:          cfg.Connection.Host,
					ExternalPort:  cfg.Ports.PgAdmin,
					InternalPort:  containerInternalPort(containerByName, cfg.Services.PgAdminContainer, cfg.Ports.PgAdmin),
					Email:         cfg.Connection.PgAdminEmail,
					Password:      cfg.Connection.PgAdminPassword,
					ServerMode:    pgAdminModeLabel(cfg.Services.PgAdmin.ServerMode),
					URL:           cfg.URLs.PgAdmin,
				}
			},
			ConnectionEntries: func(cfg configpkg.Config) []connectionEntry {
				if !cfg.PgAdminEnabled() || cfg.URLs.PgAdmin == "" {
					return nil
				}
				return []connectionEntry{{Name: "pgAdmin", Value: cfg.URLs.PgAdmin}}
			},
			CopyTargets: func() []serviceCopySpec {
				return []serviceCopySpec{
					{
						PrimaryAlias: "pgadmin",
						Aliases:      []string{"pgadminurl"},
						Label:        "pgAdmin URL",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.PgAdminEnabled() || cfg.URLs.PgAdmin == "" {
								return "", errors.New("pgadmin is not enabled in this stack")
							}
							return cfg.URLs.PgAdmin, nil
						},
					},
					{
						PrimaryAlias: "pgadmin-email",
						Aliases:      []string{"pgadminemail"},
						Label:        "pgAdmin email",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.PgAdminEnabled() {
								return "", errors.New("pgadmin is not enabled in this stack")
							}
							return cfg.Connection.PgAdminEmail, nil
						},
					},
					{
						PrimaryAlias: "pgadmin-password",
						Aliases:      []string{"pgadminpassword"},
						Label:        "pgAdmin password",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.PgAdminEnabled() {
								return "", errors.New("pgadmin is not enabled in this stack")
							}
							return cfg.Connection.PgAdminPassword, nil
						},
					},
				}
			},
		},
		{
			Key:                 "cockpit",
			DisplayName:         "Cockpit",
			Icon:                "🖥️",
			Aliases:             []string{"cockpit"},
			Kind:                serviceKindHostTool,
			Enabled:             func(cfg configpkg.Config) bool { return cfg.CockpitEnabled() },
			PrimaryPort:         func(cfg configpkg.Config) int { return cfg.Ports.Cockpit },
			PrimaryPortLabel:    "cockpit port listening",
			DefaultInternalPort: 9090,
			WaitOnStart:         false,
			BuildRuntime: func(ctx context.Context, cfg configpkg.Config, _ map[string]system.Container) runtimeService {
				cockpit := deps.cockpitStatus(ctx)
				return runtimeService{
					Name:         "cockpit",
					Icon:         "🖥️",
					DisplayName:  "Cockpit",
					Status:       cockpitStateLabel(cockpit),
					Host:         cfg.Connection.Host,
					ExternalPort: cfg.Ports.Cockpit,
					URL:          cfg.URLs.Cockpit,
				}
			},
			ConnectionEntries: func(cfg configpkg.Config) []connectionEntry {
				if cfg.URLs.Cockpit == "" {
					return nil
				}
				return []connectionEntry{{Name: "Cockpit", Value: cfg.URLs.Cockpit}}
			},
			CopyTargets: func() []serviceCopySpec {
				return []serviceCopySpec{
					{
						PrimaryAlias: "cockpit",
						Aliases:      []string{"cockpiturl"},
						Label:        "Cockpit URL",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.CockpitEnabled() {
								return "", errors.New("cockpit is not enabled in this stack")
							}
							return cfg.URLs.Cockpit, nil
						},
					},
				}
			},
		},
	}
}

func enabledServiceDefinitions(cfg configpkg.Config) []serviceDefinition {
	enabled := make([]serviceDefinition, 0, len(serviceDefinitions()))
	for _, definition := range serviceDefinitions() {
		if definition.Enabled(cfg) {
			enabled = append(enabled, definition)
		}
	}
	return enabled
}

func enabledStackServiceDefinitions(cfg configpkg.Config) []serviceDefinition {
	enabled := make([]serviceDefinition, 0, len(serviceDefinitions()))
	for _, definition := range serviceDefinitions() {
		if definition.Kind != serviceKindStack || !definition.Enabled(cfg) {
			continue
		}
		enabled = append(enabled, definition)
	}
	return enabled
}

func serviceDefinitionByAlias(alias string) (serviceDefinition, bool) {
	for _, definition := range serviceDefinitions() {
		for _, candidate := range definition.Aliases {
			if normalizedCopyTarget(candidate) == normalizedCopyTarget(alias) {
				return definition, true
			}
		}
	}
	return serviceDefinition{}, false
}

func validStackServiceNames() string {
	names := make([]string, 0, len(serviceDefinitions()))
	for _, definition := range serviceDefinitions() {
		if definition.Kind != serviceKindStack {
			continue
		}
		names = append(names, definition.Key)
	}
	return stringsJoin(names, ", ")
}

func validCopyTargetNames() string {
	names := make([]string, 0, 12)
	for _, definition := range serviceDefinitions() {
		for _, target := range definition.CopyTargets() {
			names = append(names, target.PrimaryAlias)
		}
	}
	return stringsJoin(names, ", ")
}

func stringsJoin(values []string, separator string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for _, value := range values[1:] {
		result += separator + value
	}
	return result
}

func copyTargetSpec(cfg configpkg.Config, target string) (serviceCopySpec, bool) {
	normalized := normalizedCopyTarget(target)
	for _, definition := range serviceDefinitions() {
		for _, spec := range definition.CopyTargets() {
			if normalizedCopyTarget(spec.PrimaryAlias) == normalized {
				return spec, true
			}
			for _, alias := range spec.Aliases {
				if normalizedCopyTarget(alias) == normalized {
					return spec, true
				}
			}
		}
	}
	return serviceCopySpec{}, false
}

func runtimeServiceDefinitions(cfg configpkg.Config) []serviceDefinition {
	return enabledServiceDefinitions(cfg)
}

func portMappingDefinitions(cfg configpkg.Config) []serviceDefinition {
	return enabledServiceDefinitions(cfg)
}

func waitableServiceDefinitions(cfg configpkg.Config) []serviceDefinition {
	definitions := make([]serviceDefinition, 0, len(serviceDefinitions()))
	for _, definition := range enabledStackServiceDefinitions(cfg) {
		if definition.WaitOnStart {
			definitions = append(definitions, definition)
		}
	}
	return definitions
}

func runtimeServiceForDefinition(ctx context.Context, cfg configpkg.Config, definition serviceDefinition, containerByName map[string]system.Container) runtimeService {
	return definition.BuildRuntime(ctx, cfg, containerByName)
}

func serviceDefinitionByKey(key string) (serviceDefinition, bool) {
	for _, definition := range serviceDefinitions() {
		if definition.Key == key {
			return definition, true
		}
	}
	return serviceDefinition{}, false
}

func ensureServiceEnabled(cfg configpkg.Config, service string) error {
	definition, ok := serviceDefinitionByAlias(service)
	if !ok || definition.Kind != serviceKindStack {
		return fmt.Errorf("invalid service %q; valid values: %s", service, validStackServiceNames())
	}
	if !definition.Enabled(cfg) {
		return fmt.Errorf("%s is not enabled in this stack", definition.Key)
	}
	return nil
}
