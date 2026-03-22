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
