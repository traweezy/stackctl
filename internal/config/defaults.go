package config

func Default() Config {
	cfg := Config{
		Stack: StackConfig{
			Name:        "dev-stack",
			Dir:         DefaultStackDir(),
			ComposeFile: "compose.yaml",
		},
		Services: ServicesConfig{
			PostgresContainer: "local-postgres",
			RedisContainer:    "local-redis",
			PgAdminContainer:  "local-pgadmin",
		},
		Ports: PortsConfig{
			Postgres: 5432,
			Redis:    6379,
			PgAdmin:  8081,
			Cockpit:  9090,
		},
		Behavior: BehaviorConfig{
			OpenCockpitOnStart:   true,
			OpenPgAdminOnStart:   false,
			WaitForServicesStart: true,
			StartupTimeoutSec:    30,
		},
		Setup: SetupConfig{
			InstallCockpit: true,
			IncludePgAdmin: true,
		},
		System: SystemConfig{
			PackageManager: "apt",
		},
	}

	cfg.ApplyDerivedFields()

	return cfg
}
