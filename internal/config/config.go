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
	Stack    StackConfig    `yaml:"stack"`
	Services ServicesConfig `yaml:"services"`
	Ports    PortsConfig    `yaml:"ports"`
	URLs     URLsConfig     `yaml:"urls"`
	Behavior BehaviorConfig `yaml:"behavior"`
	Setup    SetupConfig    `yaml:"setup"`
	System   SystemConfig   `yaml:"system"`
}

type StackConfig struct {
	Name        string `yaml:"name"`
	Dir         string `yaml:"dir"`
	ComposeFile string `yaml:"compose_file"`
	Managed     bool   `yaml:"managed"`
}

type ServicesConfig struct {
	PostgresContainer string `yaml:"postgres_container"`
	RedisContainer    string `yaml:"redis_container"`
	PgAdminContainer  string `yaml:"pgadmin_container"`
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

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
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
	if c.Ports.Cockpit > 0 {
		c.URLs.Cockpit = fmt.Sprintf("https://localhost:%d", c.Ports.Cockpit)
	}
	if c.Ports.PgAdmin > 0 {
		c.URLs.PgAdmin = fmt.Sprintf("http://localhost:%d", c.Ports.PgAdmin)
	}
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	return ConfigFilePath()
}
