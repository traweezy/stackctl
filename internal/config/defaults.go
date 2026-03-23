package config

func Default() Config {
	return DefaultForStack(DefaultStackName)
}

func DefaultForStack(stackName string) Config {
	name := normalizeStackName(stackName)
	cfg := Config{
		Stack: StackConfig{
			Name:        name,
			Managed:     true,
		},
		Services: ServicesConfig{
			Postgres: PostgresServiceConfig{
				Image:               "docker.io/library/postgres:16",
				MaintenanceDatabase: "postgres",
			},
			Redis: RedisServiceConfig{
				Image:           "docker.io/library/redis:7",
				AppendOnly:      false,
				SavePolicy:      "3600 1 300 100 60 10000",
				MaxMemoryPolicy: "noeviction",
			},
			NATS: NATSServiceConfig{
				Image: "docker.io/library/nats:2.12.5",
			},
			PgAdmin: PgAdminServiceConfig{
				Image:      "docker.io/dpage/pgadmin4:latest",
				ServerMode: false,
			},
		},
		Connection: ConnectionConfig{
			Host:             "localhost",
			PostgresDatabase: "app",
			PostgresUsername: "app",
			PostgresPassword: "app",
			RedisPassword:    "",
			NATSToken:        "stackctl",
			PgAdminEmail:     "admin@example.com",
			PgAdminPassword:  "admin",
		},
		Ports: PortsConfig{
			Postgres: 5432,
			Redis:    6379,
			NATS:     4222,
			PgAdmin:  8081,
			Cockpit:  9090,
		},
		Behavior: BehaviorConfig{
			WaitForServicesStart: true,
			StartupTimeoutSec:    30,
		},
		Setup: SetupConfig{
			IncludePostgres:      true,
			IncludeRedis:         true,
			IncludeCockpit:       true,
			InstallCockpit:       true,
			IncludeNATS:          true,
			IncludePgAdmin:       true,
			ScaffoldDefaultStack: true,
		},
		TUI: TUIConfig{
			AutoRefreshIntervalSec: DefaultTUIAutoRefreshIntervalSeconds,
		},
		System: SystemConfig{
			PackageManager: "apt",
		},
	}

	cfg.ApplyDerivedFields()

	return cfg
}
