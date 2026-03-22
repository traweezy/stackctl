package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var ErrNotFound = errors.New("stackctl config not found")

type Config struct {
	Stack      StackConfig      `yaml:"stack"`
	Services   ServicesConfig   `yaml:"services"`
	Connection ConnectionConfig `yaml:"connection"`
	Ports      PortsConfig      `yaml:"ports"`
	URLs       URLsConfig       `yaml:"urls"`
	Behavior   BehaviorConfig   `yaml:"behavior"`
	Setup      SetupConfig      `yaml:"setup"`
	System     SystemConfig     `yaml:"system"`
}

type StackConfig struct {
	Name        string `yaml:"name"`
	Dir         string `yaml:"dir"`
	ComposeFile string `yaml:"compose_file"`
	Managed     bool   `yaml:"managed"`
}

type ServicesConfig struct {
	PostgresContainer string                `yaml:"postgres_container"`
	RedisContainer    string                `yaml:"redis_container"`
	PgAdminContainer  string                `yaml:"pgadmin_container"`
	Postgres          PostgresServiceConfig `yaml:"postgres"`
	Redis             RedisServiceConfig    `yaml:"redis"`
	PgAdmin           PgAdminServiceConfig  `yaml:"pgadmin"`
}

type PostgresServiceConfig struct {
	Image               string `yaml:"image"`
	DataVolume          string `yaml:"data_volume"`
	MaintenanceDatabase string `yaml:"maintenance_database"`
}

type RedisServiceConfig struct {
	Image           string `yaml:"image"`
	DataVolume      string `yaml:"data_volume"`
	AppendOnly      bool   `yaml:"appendonly"`
	SavePolicy      string `yaml:"save_policy"`
	MaxMemoryPolicy string `yaml:"maxmemory_policy"`
}

type PgAdminServiceConfig struct {
	Image      string `yaml:"image"`
	DataVolume string `yaml:"data_volume"`
	ServerMode bool   `yaml:"server_mode"`
}

type ConnectionConfig struct {
	Host             string `yaml:"host"`
	PostgresDatabase string `yaml:"postgres_database"`
	PostgresUsername string `yaml:"postgres_username"`
	PostgresPassword string `yaml:"postgres_password"`
	RedisPassword    string `yaml:"redis_password"`
	PgAdminEmail     string `yaml:"pgadmin_email"`
	PgAdminPassword  string `yaml:"pgadmin_password"`
}

type PortsConfig struct {
	Postgres int `yaml:"postgres"`
	Redis    int `yaml:"redis"`
	PgAdmin  int `yaml:"pgadmin"`
	Cockpit  int `yaml:"cockpit"`
}

type URLsConfig struct {
	Cockpit string `yaml:"cockpit"`
	PgAdmin string `yaml:"pgadmin"`
}

type BehaviorConfig struct {
	WaitForServicesStart bool `yaml:"wait_for_services_on_start"`
	StartupTimeoutSec    int  `yaml:"startup_timeout_seconds"`
}

type SetupConfig struct {
	InstallCockpit       bool `yaml:"install_cockpit"`
	IncludePgAdmin       bool `yaml:"include_pgadmin"`
	ScaffoldDefaultStack bool `yaml:"scaffold_default_stack"`
}

type SystemConfig struct {
	PackageManager string `yaml:"package_manager"`
}

func Load(path string) (Config, error) {
	resolvedPath, err := resolvePath(path)
	if err != nil {
		return Config{}, err
	}

	// #nosec G304 -- config paths are chosen explicitly by the local CLI user.
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, ErrNotFound
		}
		return Config{}, fmt.Errorf("read config %q: %w", resolvedPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", resolvedPath, err)
	}

	cfg.ApplyDerivedFields()

	return cfg, nil
}

func Save(path string, cfg Config) error {
	resolvedPath, err := resolvePath(path)
	if err != nil {
		return err
	}

	cfg.ApplyDerivedFields()

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o750); err != nil {
		return fmt.Errorf("create config directory for %q: %w", resolvedPath, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(resolvedPath, data, 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", resolvedPath, err)
	}

	return nil
}

func Marshal(cfg Config) ([]byte, error) {
	cfg.ApplyDerivedFields()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	return data, nil
}

func (c *Config) ApplyDerivedFields() {
	if c.Connection.Host == "" {
		c.Connection.Host = "localhost"
	}
	if c.Connection.PostgresDatabase == "" {
		c.Connection.PostgresDatabase = "app"
	}
	if c.Connection.PostgresUsername == "" {
		c.Connection.PostgresUsername = "app"
	}
	if c.Connection.PostgresPassword == "" {
		c.Connection.PostgresPassword = "app"
	}
	if c.Connection.PgAdminEmail == "" {
		c.Connection.PgAdminEmail = "admin@example.com"
	}
	if c.Connection.PgAdminPassword == "" {
		c.Connection.PgAdminPassword = "admin"
	}

	if c.Services.Postgres.Image == "" {
		c.Services.Postgres.Image = "docker.io/library/postgres:16"
	}
	if c.Services.Postgres.DataVolume == "" {
		c.Services.Postgres.DataVolume = "postgres_data"
	}
	if c.Services.Postgres.MaintenanceDatabase == "" {
		c.Services.Postgres.MaintenanceDatabase = "postgres"
	}

	if c.Services.Redis.Image == "" {
		c.Services.Redis.Image = "docker.io/library/redis:7"
	}
	if c.Services.Redis.DataVolume == "" {
		c.Services.Redis.DataVolume = "redis_data"
	}
	if c.Services.Redis.SavePolicy == "" {
		c.Services.Redis.SavePolicy = "3600 1 300 100 60 10000"
	}
	if c.Services.Redis.MaxMemoryPolicy == "" {
		c.Services.Redis.MaxMemoryPolicy = "noeviction"
	}

	if c.Services.PgAdmin.Image == "" {
		c.Services.PgAdmin.Image = "docker.io/dpage/pgadmin4:latest"
	}
	if c.Services.PgAdmin.DataVolume == "" {
		c.Services.PgAdmin.DataVolume = "pgadmin_data"
	}

	if c.Ports.Cockpit > 0 {
		c.URLs.Cockpit = fmt.Sprintf("https://%s:%d", c.Connection.Host, c.Ports.Cockpit)
	}
	if c.Ports.PgAdmin > 0 {
		c.URLs.PgAdmin = fmt.Sprintf("http://%s:%d", c.Connection.Host, c.Ports.PgAdmin)
	}
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	return ConfigFilePath()
}
