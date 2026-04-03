package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/traweezy/stackctl/internal/logging"
	"github.com/traweezy/stackctl/internal/system"
)

var ErrNotFound = errors.New("stackctl config not found")

type Config struct {
	SchemaVersion int              `yaml:"schema_version"`
	Stack         StackConfig      `yaml:"stack"`
	Services      ServicesConfig   `yaml:"services"`
	Connection    ConnectionConfig `yaml:"connection"`
	Ports         PortsConfig      `yaml:"ports"`
	URLs          URLsConfig       `yaml:"urls"`
	Behavior      BehaviorConfig   `yaml:"behavior"`
	Setup         SetupConfig      `yaml:"setup"`
	TUI           TUIConfig        `yaml:"tui"`
	System        SystemConfig     `yaml:"system"`
}

const (
	CurrentSchemaVersion                 = 1
	DefaultTUIAutoRefreshIntervalSeconds = 30
)

type StackConfig struct {
	Name        string `yaml:"name"`
	Dir         string `yaml:"dir"`
	ComposeFile string `yaml:"compose_file"`
	Managed     bool   `yaml:"managed"`
}

type ServicesConfig struct {
	PostgresContainer    string                   `yaml:"postgres_container"`
	RedisContainer       string                   `yaml:"redis_container"`
	NATSContainer        string                   `yaml:"nats_container"`
	SeaweedFSContainer   string                   `yaml:"seaweedfs_container"`
	MeilisearchContainer string                   `yaml:"meilisearch_container"`
	PgAdminContainer     string                   `yaml:"pgadmin_container"`
	Postgres             PostgresServiceConfig    `yaml:"postgres"`
	Redis                RedisServiceConfig       `yaml:"redis"`
	NATS                 NATSServiceConfig        `yaml:"nats"`
	SeaweedFS            SeaweedFSServiceConfig   `yaml:"seaweedfs"`
	Meilisearch          MeilisearchServiceConfig `yaml:"meilisearch"`
	PgAdmin              PgAdminServiceConfig     `yaml:"pgadmin"`
}

type PostgresServiceConfig struct {
	Image                     string `yaml:"image"`
	DataVolume                string `yaml:"data_volume"`
	MaintenanceDatabase       string `yaml:"maintenance_database"`
	MaxConnections            int    `yaml:"max_connections"`
	SharedBuffers             string `yaml:"shared_buffers"`
	LogMinDurationStatementMS int    `yaml:"log_min_duration_statement_ms"`
}

type RedisServiceConfig struct {
	Image           string `yaml:"image"`
	DataVolume      string `yaml:"data_volume"`
	AppendOnly      bool   `yaml:"appendonly"`
	SavePolicy      string `yaml:"save_policy"`
	MaxMemoryPolicy string `yaml:"maxmemory_policy"`
}

type NATSServiceConfig struct {
	Image string `yaml:"image"`
}

type SeaweedFSServiceConfig struct {
	Image             string `yaml:"image"`
	DataVolume        string `yaml:"data_volume"`
	VolumeSizeLimitMB int    `yaml:"volume_size_limit_mb"`
}

type MeilisearchServiceConfig struct {
	Image      string `yaml:"image"`
	DataVolume string `yaml:"data_volume"`
}

type PgAdminServiceConfig struct {
	Image                   string `yaml:"image"`
	DataVolume              string `yaml:"data_volume"`
	ServerMode              bool   `yaml:"server_mode"`
	BootstrapPostgresServer bool   `yaml:"bootstrap_postgres_server"`
	BootstrapServerName     string `yaml:"bootstrap_server_name"`
	BootstrapServerGroup    string `yaml:"bootstrap_server_group"`
}

type ConnectionConfig struct {
	Host                 string `yaml:"host"`
	PostgresDatabase     string `yaml:"postgres_database"`
	PostgresUsername     string `yaml:"postgres_username"`
	PostgresPassword     string `yaml:"postgres_password"`
	RedisPassword        string `yaml:"redis_password"`
	RedisACLUsername     string `yaml:"redis_acl_username"`
	RedisACLPassword     string `yaml:"redis_acl_password"`
	NATSToken            string `yaml:"nats_token"`
	SeaweedFSAccessKey   string `yaml:"seaweedfs_access_key"`
	SeaweedFSSecretKey   string `yaml:"seaweedfs_secret_key"`
	MeilisearchMasterKey string `yaml:"meilisearch_master_key"`
	PgAdminEmail         string `yaml:"pgadmin_email"`
	PgAdminPassword      string `yaml:"pgadmin_password"`
}

type PortsConfig struct {
	Postgres    int `yaml:"postgres"`
	Redis       int `yaml:"redis"`
	NATS        int `yaml:"nats"`
	SeaweedFS   int `yaml:"seaweedfs"`
	Meilisearch int `yaml:"meilisearch"`
	PgAdmin     int `yaml:"pgadmin"`
	Cockpit     int `yaml:"cockpit"`
}

type URLsConfig struct {
	SeaweedFS   string `yaml:"seaweedfs"`
	Meilisearch string `yaml:"meilisearch"`
	Cockpit     string `yaml:"cockpit"`
	PgAdmin     string `yaml:"pgadmin"`
}

type BehaviorConfig struct {
	WaitForServicesStart bool `yaml:"wait_for_services_on_start"`
	StartupTimeoutSec    int  `yaml:"startup_timeout_seconds"`
}

type SetupConfig struct {
	IncludePostgres      bool `yaml:"include_postgres"`
	IncludeRedis         bool `yaml:"include_redis"`
	IncludeCockpit       bool `yaml:"include_cockpit"`
	InstallCockpit       bool `yaml:"install_cockpit"`
	IncludeNATS          bool `yaml:"include_nats"`
	IncludeSeaweedFS     bool `yaml:"include_seaweedfs"`
	IncludeMeilisearch   bool `yaml:"include_meilisearch"`
	IncludePgAdmin       bool `yaml:"include_pgadmin"`
	ScaffoldDefaultStack bool `yaml:"scaffold_default_stack"`
}

type TUIConfig struct {
	AutoRefreshIntervalSec int `yaml:"auto_refresh_interval_seconds"`
}

type SystemConfig struct {
	PackageManager string `yaml:"package_manager"`
}

func Load(path string) (Config, error) {
	return loadWithPlatform(path, system.CurrentPlatform())
}

func loadWithPlatform(path string, platform system.Platform) (Config, error) {
	resolvedPath, err := resolvePath(path)
	if err != nil {
		return Config{}, err
	}
	log := logging.With("component", "config", "path", resolvedPath)
	log.Debug("loading config")

	// #nosec G304 -- config paths are chosen explicitly by the local CLI user.
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Debug("config file not found")
			return Config{}, ErrNotFound
		}
		log.Error("config read failed", "error", err)
		return Config{}, fmt.Errorf("read config %q: %w", resolvedPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Error("config parse failed", "error", err)
		return Config{}, fmt.Errorf("parse config %q: %w", resolvedPath, err)
	}
	applyLegacySetupDefaults(data, &cfg, platform)
	if err := cfg.normalizeSchemaVersion(); err != nil {
		log.Error("config schema validation failed", "error", err)
		return Config{}, fmt.Errorf("validate config schema for %q: %w", resolvedPath, err)
	}

	cfg.ApplyDerivedFields()
	log.Debug("config loaded", "stack", cfg.Stack.Name, "managed", cfg.Stack.Managed, "bytes", len(data))

	return cfg, nil
}

func Save(path string, cfg Config) error {
	resolvedPath, err := resolvePath(path)
	if err != nil {
		return err
	}
	log := logging.With("component", "config", "path", resolvedPath)

	cfg.ApplyDerivedFields()

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o750); err != nil {
		log.Error("config directory create failed", "error", err)
		return fmt.Errorf("create config directory for %q: %w", resolvedPath, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		log.Error("config marshal failed", "error", err)
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(resolvedPath, data, 0o600); err != nil {
		log.Error("config write failed", "error", err)
		return fmt.Errorf("write config %q: %w", resolvedPath, err)
	}
	log.Debug("config saved", "stack", cfg.Stack.Name, "managed", cfg.Stack.Managed, "bytes", len(data))

	return nil
}

func Marshal(cfg Config) ([]byte, error) {
	cfg.ApplyDerivedFields()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		logging.With("component", "config").Error("config marshal failed", "error", err)
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	logging.With("component", "config").Debug("config marshaled", "stack", cfg.Stack.Name, "managed", cfg.Stack.Managed, "bytes", len(data))

	return data, nil
}

func (c *Config) ApplyDerivedFields() {
	c.SchemaVersion = CurrentSchemaVersion
	c.Stack.Name = normalizeStackName(c.Stack.Name)
	if c.Stack.Managed {
		if managedDir, err := ManagedStackDir(c.Stack.Name); err == nil {
			c.Stack.Dir = managedDir
		}
		if strings.TrimSpace(c.Stack.ComposeFile) == "" {
			c.Stack.ComposeFile = DefaultComposeFileName
		}
	}
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
	if c.Connection.NATSToken == "" {
		c.Connection.NATSToken = "stackctl"
	}
	if c.Connection.SeaweedFSAccessKey == "" {
		c.Connection.SeaweedFSAccessKey = "stackctl"
	}
	if c.Connection.SeaweedFSSecretKey == "" {
		c.Connection.SeaweedFSSecretKey = "stackctlsecret"
	}
	if c.Connection.MeilisearchMasterKey == "" {
		c.Connection.MeilisearchMasterKey = "stackctl-meili-master-key"
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
	if c.Services.PostgresContainer == "" {
		c.Services.PostgresContainer = defaultPostgresContainerName(c.Stack.Name)
	}
	if c.Services.Postgres.DataVolume == "" {
		c.Services.Postgres.DataVolume = defaultPostgresVolumeName(c.Stack.Name)
	}
	if c.Services.Postgres.MaintenanceDatabase == "" {
		c.Services.Postgres.MaintenanceDatabase = "postgres"
	}
	if c.Services.Postgres.MaxConnections <= 0 {
		c.Services.Postgres.MaxConnections = 100
	}
	if c.Services.Postgres.SharedBuffers == "" {
		c.Services.Postgres.SharedBuffers = "128MB"
	}
	if c.Services.Postgres.LogMinDurationStatementMS == 0 {
		c.Services.Postgres.LogMinDurationStatementMS = -1
	}

	if c.Services.Redis.Image == "" {
		c.Services.Redis.Image = "docker.io/library/redis:7"
	}
	if c.Services.RedisContainer == "" {
		c.Services.RedisContainer = defaultRedisContainerName(c.Stack.Name)
	}
	if c.Services.Redis.DataVolume == "" {
		c.Services.Redis.DataVolume = defaultRedisVolumeName(c.Stack.Name)
	}
	if c.Services.Redis.SavePolicy == "" {
		c.Services.Redis.SavePolicy = "3600 1 300 100 60 10000"
	}
	if c.Services.Redis.MaxMemoryPolicy == "" {
		c.Services.Redis.MaxMemoryPolicy = "noeviction"
	}
	if c.Services.NATS.Image == "" {
		c.Services.NATS.Image = "docker.io/library/nats:2.12.5"
	}
	if c.Services.NATSContainer == "" {
		c.Services.NATSContainer = defaultNATSContainerName(c.Stack.Name)
	}

	if c.Services.SeaweedFS.Image == "" {
		c.Services.SeaweedFS.Image = "docker.io/chrislusf/seaweedfs:4.17@sha256:186de7ef977a20343ee9a5544073f081976a29e2d29ecf8379891e7bf177fbe9"
	}
	if c.Services.SeaweedFSContainer == "" {
		c.Services.SeaweedFSContainer = defaultSeaweedFSContainerName(c.Stack.Name)
	}
	if c.Services.SeaweedFS.DataVolume == "" {
		c.Services.SeaweedFS.DataVolume = defaultSeaweedFSVolumeName(c.Stack.Name)
	}
	if c.Services.SeaweedFS.VolumeSizeLimitMB <= 0 {
		c.Services.SeaweedFS.VolumeSizeLimitMB = 1024
	}

	if c.Services.Meilisearch.Image == "" {
		c.Services.Meilisearch.Image = "docker.io/getmeili/meilisearch:v1.40.0"
	}
	if c.Services.MeilisearchContainer == "" {
		c.Services.MeilisearchContainer = defaultMeilisearchContainerName(c.Stack.Name)
	}
	if c.Services.Meilisearch.DataVolume == "" {
		c.Services.Meilisearch.DataVolume = defaultMeilisearchVolumeName(c.Stack.Name)
	}

	if c.Services.PgAdmin.Image == "" {
		c.Services.PgAdmin.Image = "docker.io/dpage/pgadmin4:latest"
	}
	if c.Services.PgAdminContainer == "" {
		c.Services.PgAdminContainer = defaultPgAdminContainerName(c.Stack.Name)
	}
	if c.Services.PgAdmin.DataVolume == "" {
		c.Services.PgAdmin.DataVolume = defaultPgAdminVolumeName(c.Stack.Name)
	}
	if c.Services.PgAdmin.BootstrapServerName == "" {
		c.Services.PgAdmin.BootstrapServerName = "Local Postgres"
	}
	if c.Services.PgAdmin.BootstrapServerGroup == "" {
		c.Services.PgAdmin.BootstrapServerGroup = "Local"
	}

	if c.Ports.Cockpit > 0 {
		c.URLs.Cockpit = fmt.Sprintf("https://%s:%d", c.Connection.Host, c.Ports.Cockpit)
	}
	if c.Ports.SeaweedFS > 0 {
		c.URLs.SeaweedFS = fmt.Sprintf("http://%s:%d", c.Connection.Host, c.Ports.SeaweedFS)
	}
	if c.Ports.Meilisearch > 0 {
		c.URLs.Meilisearch = fmt.Sprintf("http://%s:%d", c.Connection.Host, c.Ports.Meilisearch)
	}
	if c.Ports.PgAdmin > 0 {
		c.URLs.PgAdmin = fmt.Sprintf("http://%s:%d", c.Connection.Host, c.Ports.PgAdmin)
	}
	if c.TUI.AutoRefreshIntervalSec <= 0 {
		c.TUI.AutoRefreshIntervalSec = DefaultTUIAutoRefreshIntervalSeconds
	}
}

func (c *Config) normalizeSchemaVersion() error {
	switch c.SchemaVersion {
	case 0:
		c.SchemaVersion = CurrentSchemaVersion
		return nil
	case CurrentSchemaVersion:
		return nil
	default:
		return fmt.Errorf("unsupported schema_version %d (current %d)", c.SchemaVersion, CurrentSchemaVersion)
	}
}

func (c Config) PostgresEnabled() bool {
	return c.Setup.IncludePostgres
}

func (c Config) RedisEnabled() bool {
	return c.Setup.IncludeRedis
}

func (c Config) NATSEnabled() bool {
	return c.Setup.IncludeNATS
}

func (c Config) SeaweedFSEnabled() bool {
	return c.Setup.IncludeSeaweedFS
}

func (c Config) MeilisearchEnabled() bool {
	return c.Setup.IncludeMeilisearch
}

func (c Config) PgAdminEnabled() bool {
	return c.Setup.IncludePgAdmin
}

func (c Config) RedisACLEnabled() bool {
	return strings.TrimSpace(c.Connection.RedisACLUsername) != "" && strings.TrimSpace(c.Connection.RedisACLPassword) != ""
}

func (c Config) PgAdminBootstrapEnabled() bool {
	return c.PgAdminEnabled() && c.PostgresEnabled() && c.Services.PgAdmin.BootstrapPostgresServer
}

func (c Config) CockpitEnabled() bool {
	return c.Setup.IncludeCockpit
}

func (c Config) EnabledStackServiceCount() int {
	count := 0
	if c.PostgresEnabled() {
		count++
	}
	if c.RedisEnabled() {
		count++
	}
	if c.NATSEnabled() {
		count++
	}
	if c.SeaweedFSEnabled() {
		count++
	}
	if c.MeilisearchEnabled() {
		count++
	}
	if c.PgAdminEnabled() {
		count++
	}
	return count
}

func applyLegacySetupDefaults(data []byte, cfg *Config, platform system.Platform) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return
	}

	if !yamlPathPresent(&root, "setup", "include_postgres") {
		cfg.Setup.IncludePostgres = true
	}
	if !yamlPathPresent(&root, "setup", "include_redis") {
		cfg.Setup.IncludeRedis = true
	}
	if !yamlPathPresent(&root, "setup", "include_cockpit") {
		cfg.Setup.IncludeCockpit = platform.SupportsCockpit()
	}
	if !yamlPathPresent(&root, "setup", "include_nats") {
		cfg.Setup.IncludeNATS = true
	}
	if !yamlPathPresent(&root, "setup", "include_pgadmin") {
		cfg.Setup.IncludePgAdmin = true
	}
	if !yamlPathPresent(&root, "setup", "install_cockpit") {
		cfg.Setup.InstallCockpit = platform.SupportsCockpit()
	}
	if !yamlPathPresent(&root, "setup", "scaffold_default_stack") {
		cfg.Setup.ScaffoldDefaultStack = true
	}
	if !yamlPathPresent(&root, "system", "package_manager") {
		if packageManager := strings.TrimSpace(platform.PackageManager); packageManager != "" {
			cfg.System.PackageManager = packageManager
		}
	}
	if !yamlPathPresent(&root, "services", "postgres", "max_connections") {
		cfg.Services.Postgres.MaxConnections = 100
	}
	if !yamlPathPresent(&root, "services", "postgres", "shared_buffers") {
		cfg.Services.Postgres.SharedBuffers = "128MB"
	}
	if !yamlPathPresent(&root, "services", "postgres", "log_min_duration_statement_ms") {
		cfg.Services.Postgres.LogMinDurationStatementMS = -1
	}
	if !yamlPathPresent(&root, "services", "pgadmin", "bootstrap_server_name") {
		cfg.Services.PgAdmin.BootstrapServerName = "Local Postgres"
	}
	if !yamlPathPresent(&root, "services", "pgadmin", "bootstrap_server_group") {
		cfg.Services.PgAdmin.BootstrapServerGroup = "Local"
	}
	if !yamlPathPresent(&root, "services", "pgadmin", "bootstrap_postgres_server") {
		cfg.Services.PgAdmin.BootstrapPostgresServer = true
	}
}

func yamlPathPresent(node *yaml.Node, keys ...string) bool {
	if node == nil || len(keys) == 0 {
		return false
	}

	current := node
	if current.Kind == yaml.DocumentNode && len(current.Content) > 0 {
		current = current.Content[0]
	}

	for _, key := range keys {
		if current == nil || current.Kind != yaml.MappingNode {
			return false
		}

		next := (*yaml.Node)(nil)
		for idx := 0; idx+1 < len(current.Content); idx += 2 {
			if current.Content[idx].Value == key {
				next = current.Content[idx+1]
				break
			}
		}
		if next == nil {
			return false
		}
		current = next
	}

	return true
}

func resolvePath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	return ConfigFilePath()
}
