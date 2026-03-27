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
	EnvEntries          func(configpkg.Config) []envEntry
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
					InternalPort:  resolvedContainerInternalPort(containerByName, cfg.Services.PostgresContainer, cfg.Ports.Postgres, 5432),
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
			EnvEntries: func(cfg configpkg.Config) []envEntry {
				return []envEntry{
					{Name: "DATABASE_URL", Value: postgresDSN(cfg)},
					{Name: "POSTGRES_URL", Value: postgresDSN(cfg)},
					{Name: "PGHOST", Value: cfg.Connection.Host},
					{Name: "PGPORT", Value: fmt.Sprintf("%d", cfg.Ports.Postgres)},
					{Name: "PGDATABASE", Value: cfg.Connection.PostgresDatabase},
					{Name: "PGUSER", Value: cfg.Connection.PostgresUsername},
					{Name: "PGPASSWORD", Value: cfg.Connection.PostgresPassword},
					{Name: "POSTGRES_HOST", Value: cfg.Connection.Host},
					{Name: "POSTGRES_PORT", Value: fmt.Sprintf("%d", cfg.Ports.Postgres)},
					{Name: "POSTGRES_DB", Value: cfg.Connection.PostgresDatabase},
					{Name: "POSTGRES_USER", Value: cfg.Connection.PostgresUsername},
					{Name: "POSTGRES_PASSWORD", Value: cfg.Connection.PostgresPassword},
				}
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
					InternalPort:    resolvedContainerInternalPort(containerByName, cfg.Services.RedisContainer, cfg.Ports.Redis, 6379),
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
			EnvEntries: func(cfg configpkg.Config) []envEntry {
				return []envEntry{
					{Name: "REDIS_URL", Value: redisDSN(cfg)},
					{Name: "REDIS_HOST", Value: cfg.Connection.Host},
					{Name: "REDIS_PORT", Value: fmt.Sprintf("%d", cfg.Ports.Redis)},
					{Name: "REDIS_PASSWORD", Value: cfg.Connection.RedisPassword},
				}
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
					InternalPort:  resolvedContainerInternalPort(containerByName, cfg.Services.NATSContainer, cfg.Ports.NATS, 4222),
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
			EnvEntries: func(cfg configpkg.Config) []envEntry {
				return []envEntry{
					{Name: "NATS_URL", Value: natsDSN(cfg)},
					{Name: "NATS_HOST", Value: cfg.Connection.Host},
					{Name: "NATS_PORT", Value: fmt.Sprintf("%d", cfg.Ports.NATS)},
					{Name: "NATS_TOKEN", Value: cfg.Connection.NATSToken},
				}
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
			Key:                 "seaweedfs",
			DisplayName:         "SeaweedFS",
			Icon:                "🪣",
			Aliases:             []string{"seaweedfs", "seaweed"},
			Kind:                serviceKindStack,
			Enabled:             func(cfg configpkg.Config) bool { return cfg.SeaweedFSEnabled() },
			ContainerName:       func(cfg configpkg.Config) string { return cfg.Services.SeaweedFSContainer },
			PrimaryPort:         func(cfg configpkg.Config) int { return cfg.Ports.SeaweedFS },
			PrimaryPortLabel:    "seaweedfs s3 port listening",
			DefaultInternalPort: 8333,
			WaitOnStart:         true,
			BuildRuntime: func(_ context.Context, cfg configpkg.Config, containerByName map[string]system.Container) runtimeService {
				return runtimeService{
					Name:              "seaweedfs",
					Icon:              "🪣",
					DisplayName:       "SeaweedFS",
					Status:            containerStatus(containerByName, cfg.Services.SeaweedFSContainer),
					ContainerName:     cfg.Services.SeaweedFSContainer,
					Image:             cfg.Services.SeaweedFS.Image,
					DataVolume:        cfg.Services.SeaweedFS.DataVolume,
					Host:              cfg.Connection.Host,
					ExternalPort:      cfg.Ports.SeaweedFS,
					InternalPort:      resolvedContainerInternalPort(containerByName, cfg.Services.SeaweedFSContainer, cfg.Ports.SeaweedFS, 8333),
					AccessKey:         cfg.Connection.SeaweedFSAccessKey,
					SecretKey:         cfg.Connection.SeaweedFSSecretKey,
					VolumeSizeLimitMB: cfg.Services.SeaweedFS.VolumeSizeLimitMB,
					Endpoint:          seaweedFSEndpoint(cfg),
				}
			},
			ConnectionEntries: func(cfg configpkg.Config) []connectionEntry {
				if !cfg.SeaweedFSEnabled() {
					return nil
				}
				return []connectionEntry{
					{Name: "SeaweedFS S3 endpoint", Value: seaweedFSEndpoint(cfg)},
					{Name: "SeaweedFS access key", Value: cfg.Connection.SeaweedFSAccessKey},
					{Name: "SeaweedFS secret key", Value: cfg.Connection.SeaweedFSSecretKey},
				}
			},
			EnvEntries: func(cfg configpkg.Config) []envEntry {
				endpoint := seaweedFSEndpoint(cfg)
				return []envEntry{
					{Name: "S3_ENDPOINT", Value: endpoint},
					{Name: "AWS_ACCESS_KEY_ID", Value: cfg.Connection.SeaweedFSAccessKey},
					{Name: "AWS_SECRET_ACCESS_KEY", Value: cfg.Connection.SeaweedFSSecretKey},
					{Name: "SEAWEEDFS_ENDPOINT", Value: endpoint},
					{Name: "SEAWEEDFS_ACCESS_KEY", Value: cfg.Connection.SeaweedFSAccessKey},
					{Name: "SEAWEEDFS_SECRET_KEY", Value: cfg.Connection.SeaweedFSSecretKey},
				}
			},
			CopyTargets: func() []serviceCopySpec {
				return []serviceCopySpec{
					{
						PrimaryAlias: "seaweedfs",
						Aliases:      []string{"seaweed", "seaweedendpoint", "seaweedfsendpoint"},
						Label:        "SeaweedFS endpoint",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.SeaweedFSEnabled() {
								return "", errors.New("seaweedfs is not enabled in this stack")
							}
							return seaweedFSEndpoint(cfg), nil
						},
					},
					{
						PrimaryAlias: "seaweedfs-access-key",
						Aliases:      []string{"seaweedaccesskey", "seaweedfsaccesskey"},
						Label:        "SeaweedFS access key",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.SeaweedFSEnabled() {
								return "", errors.New("seaweedfs is not enabled in this stack")
							}
							return cfg.Connection.SeaweedFSAccessKey, nil
						},
					},
					{
						PrimaryAlias: "seaweedfs-secret-key",
						Aliases:      []string{"seaweedsecretkey", "seaweedfssecretkey"},
						Label:        "SeaweedFS secret key",
						Resolve: func(cfg configpkg.Config) (string, error) {
							if !cfg.SeaweedFSEnabled() {
								return "", errors.New("seaweedfs is not enabled in this stack")
							}
							return cfg.Connection.SeaweedFSSecretKey, nil
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
					InternalPort:  resolvedContainerInternalPort(containerByName, cfg.Services.PgAdminContainer, cfg.Ports.PgAdmin, 80),
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
			EnvEntries: func(cfg configpkg.Config) []envEntry {
				return []envEntry{
					{Name: "PGADMIN_URL", Value: cfg.URLs.PgAdmin},
					{Name: "PGADMIN_EMAIL", Value: cfg.Connection.PgAdminEmail},
					{Name: "PGADMIN_PASSWORD", Value: cfg.Connection.PgAdminPassword},
				}
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
				portState := inspectHostPort(cfg.Ports.Cockpit)
				return runtimeService{
					Name:          "cockpit",
					Icon:          "🖥️",
					DisplayName:   "Cockpit",
					Status:        cockpitRuntimeStateLabel(cockpit, portState),
					Host:          cfg.Connection.Host,
					ExternalPort:  cfg.Ports.Cockpit,
					PortListening: portState.Listening,
					PortConflict:  portState.Conflict,
					URL:           cfg.URLs.Cockpit,
				}
			},
			ConnectionEntries: func(cfg configpkg.Config) []connectionEntry {
				if cfg.URLs.Cockpit == "" {
					return nil
				}
				return []connectionEntry{{Name: "Cockpit", Value: cfg.URLs.Cockpit}}
			},
			EnvEntries: func(cfg configpkg.Config) []envEntry {
				return []envEntry{
					{Name: "COCKPIT_URL", Value: cfg.URLs.Cockpit},
				}
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

func validEnvTargetNames() string {
	names := make([]string, 0, len(serviceDefinitions()))
	for _, definition := range serviceDefinitions() {
		names = append(names, definition.Key)
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
