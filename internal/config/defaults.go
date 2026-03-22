package config

func Default() Config {
	cfg := Config{
		Stack: StackConfig{
			Name:        DefaultStackName,
			Dir:         DefaultManagedStackDir(),
			ComposeFile: DefaultComposeFileName,
			Managed:     true,
		},
		Services: ServicesConfig{
			PostgresContainer: "local-postgres",
			RedisContainer:    "local-redis",
			PgAdminContainer:  "local-pgadmin",
			Postgres: PostgresServiceConfig{
				Image:               "docker.io/library/postgres:16",
				DataVolume:          "postgres_data",
				MaintenanceDatabase: "postgres",
			},
			Redis: RedisServiceConfig{
				Image:           "docker.io/library/redis:7",
				DataVolume:      "redis_data",
				AppendOnly:      false,
				SavePolicy:      "3600 1 300 100 60 10000",
				MaxMemoryPolicy: "noeviction",
			},
			PgAdmin: PgAdminServiceConfig{
				Image:      "docker.io/dpage/pgadmin4:latest",
				DataVolume: "pgadmin_data",
				ServerMode: false,
			},
		},
		Connection: ConnectionConfig{
			Host:             "localhost",
			PostgresDatabase: "app",
			PostgresUsername: "app",
			PostgresPassword: "app",
			RedisPassword:    "",
			PgAdminEmail:     "admin@example.com",
			PgAdminPassword:  "admin",
		},
		Ports: PortsConfig{
			Postgres: 5432,
			Redis:    6379,
			PgAdmin:  8081,
			Cockpit:  9090,
		},
		Behavior: BehaviorConfig{
			WaitForServicesStart: true,
			StartupTimeoutSec:    30,
		},
		Setup: SetupConfig{
			InstallCockpit:       true,
			IncludePgAdmin:       true,
			ScaffoldDefaultStack: true,
		},
		System: SystemConfig{
			PackageManager: "apt",
		},
	}

	cfg.ApplyDerivedFields()

	return cfg
}
