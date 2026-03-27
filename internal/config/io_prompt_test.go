package config

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty/v2"
)

func TestSaveLoadAndMarshalRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Default()
	cfg.TUI.AutoRefreshIntervalSec = 12

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Stack.Name != cfg.Stack.Name {
		t.Fatalf("loaded config stack name = %q", loaded.Stack.Name)
	}
	if loaded.TUI.AutoRefreshIntervalSec != 12 {
		t.Fatalf("loaded config TUI auto-refresh interval = %d", loaded.TUI.AutoRefreshIntervalSec)
	}

	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if !strings.Contains(string(data), "stack:") {
		t.Fatalf("marshal output missing stack section: %s", string(data))
	}
	if strings.Contains(string(data), "open_cockpit_on_start") || strings.Contains(string(data), "open_pgadmin_on_start") {
		t.Fatalf("marshal output should not include removed open-on-start fields: %s", string(data))
	}
	if !strings.Contains(string(data), "auto_refresh_interval_seconds: 12") {
		t.Fatalf("marshal output missing TUI auto-refresh interval: %s", string(data))
	}
}

func TestLoadRejectsMalformedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("stack: ["), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("unexpected load error: %v", err)
	}
}

func TestLoadIgnoresLegacyOpenOnStartFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := `stack:
  name: dev-stack
  dir: /tmp/dev-stack
  compose_file: compose.yaml
  managed: false
services:
  postgres_container: local-postgres
  redis_container: local-redis
  pgadmin_container: local-pgadmin
ports:
  postgres: 5432
  redis: 6379
  pgadmin: 8081
  cockpit: 9090
urls:
  cockpit: https://localhost:9090
  pgadmin: http://localhost:8081
behavior:
  open_cockpit_on_start: true
  open_pgadmin_on_start: false
  wait_for_services_on_start: true
  startup_timeout_seconds: 30
setup:
  install_cockpit: true
  include_pgadmin: true
  scaffold_default_stack: false
system:
  package_manager: apt
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Behavior.WaitForServicesStart || cfg.Behavior.StartupTimeoutSec != 30 {
		t.Fatalf("unexpected behavior config: %+v", cfg.Behavior)
	}
}

func TestLoadAppliesDefaultTUIAutoRefreshIntervalWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := `stack:
  name: dev-stack
  dir: /tmp/dev-stack
  compose_file: compose.yaml
  managed: false
services:
  postgres_container: local-postgres
  redis_container: local-redis
  pgadmin_container: local-pgadmin
connection:
  host: localhost
  postgres_database: app
  postgres_username: app
  postgres_password: app
  redis_password: ""
  pgadmin_email: admin@example.com
  pgadmin_password: admin
ports:
  postgres: 5432
  redis: 6379
  pgadmin: 8081
  cockpit: 9090
behavior:
  wait_for_services_on_start: true
  startup_timeout_seconds: 30
setup:
  install_cockpit: true
  include_pgadmin: true
  scaffold_default_stack: false
system:
  package_manager: apt
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.TUI.AutoRefreshIntervalSec != DefaultTUIAutoRefreshIntervalSeconds {
		t.Fatalf("expected default TUI auto-refresh interval, got %+v", cfg.TUI)
	}
}

func TestLoadAppliesLegacyServiceEnableDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := `stack:
  name: dev-stack
  dir: /tmp/dev-stack
  compose_file: compose.yaml
  managed: false
services:
  postgres_container: local-postgres
  redis_container: local-redis
connection:
  host: localhost
ports:
  postgres: 5432
  redis: 6379
behavior:
  wait_for_services_on_start: true
  startup_timeout_seconds: 30
setup:
  scaffold_default_stack: false
system:
  package_manager: apt
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Setup.IncludePostgres || !cfg.Setup.IncludeRedis || !cfg.Setup.IncludeNATS || !cfg.Setup.IncludePgAdmin || !cfg.Setup.IncludeCockpit {
		t.Fatalf("expected legacy config to inherit service defaults, got %+v", cfg.Setup)
	}
	if !cfg.Setup.InstallCockpit {
		t.Fatalf("expected legacy config to default install_cockpit, got %+v", cfg.Setup)
	}
}

func TestLoadMissingConfigReturnsErrNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLoadReturnsReadErrorForDirectoryPath(t *testing.T) {
	_, err := Load(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("unexpected load error: %v", err)
	}
}

func TestSaveAndLoadUsingResolvedDefaultPath(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	cfg := Default()
	if err := Save("", cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Stack.Name != cfg.Stack.Name {
		t.Fatalf("loaded config mismatch: %+v", loaded)
	}
}

func TestConfigPathsRespectUserConfigDir(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	dirPath, err := ConfigDirPath()
	if err != nil {
		t.Fatalf("ConfigDirPath returned error: %v", err)
	}
	if dirPath != filepath.Join(configRoot, "stackctl") {
		t.Fatalf("unexpected config dir path: %s", dirPath)
	}

	filePath, err := ConfigFilePath()
	if err != nil {
		t.Fatalf("ConfigFilePath returned error: %v", err)
	}
	if filePath != filepath.Join(configRoot, "stackctl", "config.yaml") {
		t.Fatalf("unexpected config file path: %s", filePath)
	}
}

func TestNamedStackConfigPathUsesStacksSubdir(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	filePath, err := ConfigFilePathForStack("staging")
	if err != nil {
		t.Fatalf("ConfigFilePathForStack returned error: %v", err)
	}
	if filePath != filepath.Join(configRoot, "stackctl", "stacks", "staging.yaml") {
		t.Fatalf("unexpected named stack config path: %s", filePath)
	}
}

func TestConfigFilePathUsesSelectedStackEnv(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv(StackNameEnvVar, "staging")

	filePath, err := ConfigFilePath()
	if err != nil {
		t.Fatalf("ConfigFilePath returned error: %v", err)
	}
	if filePath != filepath.Join(configRoot, "stackctl", "stacks", "staging.yaml") {
		t.Fatalf("unexpected env-selected config path: %s", filePath)
	}
}

func TestKnownConfigPathsIncludesDefaultAndNamedStacks(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	defaultPath := filepath.Join(configRoot, "stackctl", "config.yaml")
	namedPath := filepath.Join(configRoot, "stackctl", "stacks", "staging.yaml")
	if err := os.MkdirAll(filepath.Dir(namedPath), 0o755); err != nil {
		t.Fatalf("mkdir stack config dir: %v", err)
	}
	if err := os.WriteFile(defaultPath, []byte("stack:\n  name: dev-stack\n"), 0o644); err != nil {
		t.Fatalf("write default config: %v", err)
	}
	if err := os.WriteFile(namedPath, []byte("stack:\n  name: staging\n"), 0o644); err != nil {
		t.Fatalf("write named config: %v", err)
	}

	paths, err := KnownConfigPaths()
	if err != nil {
		t.Fatalf("KnownConfigPaths returned error: %v", err)
	}
	if len(paths) != 2 || paths[0] != defaultPath || paths[1] != namedPath {
		t.Fatalf("unexpected known config paths: %+v", paths)
	}
}

func TestSaveCreatesNestedConfigDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected saved file to exist: %v", err)
	}
}

func TestSaveReturnsWriteErrorForDirectoryPath(t *testing.T) {
	err := Save(t.TempDir(), Default())
	if err == nil || !strings.Contains(err.Error(), "write config") {
		t.Fatalf("unexpected save error: %v", err)
	}
}

func TestComposePathAndValidateOrError(t *testing.T) {
	cfg := Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false
	cfg.Stack.Dir = filepath.Join(t.TempDir(), "stack")
	if err := os.MkdirAll(cfg.Stack.Dir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(ComposePath(cfg), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file failed: %v", err)
	}

	if got := ComposePath(cfg); got != filepath.Join(cfg.Stack.Dir, cfg.Stack.ComposeFile) {
		t.Fatalf("unexpected compose path: %s", got)
	}

	if err := ValidateOrError(cfg); err != nil {
		t.Fatalf("ValidateOrError returned error: %v", err)
	}
}

func TestValidateOrErrorReportsInvalidDirectoryFile(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("file"), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg := Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false
	cfg.Stack.Dir = filePath

	err := ValidateOrError(cfg)
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "directory does not exist") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestRunWizardAcceptsDefaults(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()

	input := strings.Repeat("\n", 48)
	got, err := RunWizard(strings.NewReader(input), io.Discard, cfg)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	wantDir, err := ManagedStackDir(cfg.Stack.Name)
	if err != nil {
		t.Fatalf("ManagedStackDir returned error: %v", err)
	}
	if got.Stack.Dir != wantDir {
		t.Fatalf("wizard changed stack dir: %s", got.Stack.Dir)
	}
	if !got.Stack.Managed || !got.Setup.ScaffoldDefaultStack {
		t.Fatalf("wizard did not keep managed stack defaults: %+v", got)
	}
	if got.URLs.Cockpit == "" || got.URLs.PgAdmin == "" {
		t.Fatalf("wizard did not derive urls: %+v", got.URLs)
	}
	if got.Connection.RedisPassword != "" {
		t.Fatalf("wizard should keep redis auth disabled by default: %+v", got.Connection)
	}
	if got.Connection.PgAdminEmail != "admin@example.com" || got.Connection.PgAdminPassword != "admin" {
		t.Fatalf("wizard did not preserve pgadmin defaults: %+v", got.Connection)
	}
}

func TestRunWizardGroupsPromptsByService(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	var out strings.Builder

	_, err := RunWizard(strings.NewReader(strings.Repeat("\n", 48)), &out, cfg)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	output := out.String()
	for _, fragment := range []string{
		"[Postgres]",
		"[Redis]",
		"[NATS]",
		"[pgAdmin]",
		"[Cockpit]",
		"[Behavior]",
		"[System]",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("wizard output missing %q:\n%s", fragment, output)
		}
	}

	assertBefore := func(first, second string) {
		t.Helper()

		firstIndex := strings.Index(output, first)
		secondIndex := strings.Index(output, second)
		if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
			t.Fatalf("expected %q before %q in wizard output:\n%s", first, second, output)
		}
	}

	assertBefore("Include Postgres in the stack", "Postgres container name")
	assertBefore("Postgres password", "Include Redis in the stack")
	assertBefore("Redis password (leave blank to disable auth)", "Include NATS in the stack")
	assertBefore("NATS auth token", "Include pgAdmin in the stack")
	assertBefore("pgAdmin password", "Include Cockpit helpers")
	assertBefore("Install Cockpit during setup", "[Behavior]")
}

func TestRunWizardCanSwitchToExternalStack(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	externalDir := filepath.Join(t.TempDir(), "external-stack")
	if err := os.MkdirAll(externalDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	cfg := Default()
	input := "dev-stack\nn\n" + externalDir + "\ncompose.custom.yaml\n" + strings.Repeat("\n", 48)

	got, err := RunWizard(strings.NewReader(input), io.Discard, cfg)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	if got.Stack.Managed {
		t.Fatalf("expected external stack config, got %+v", got.Stack)
	}
	if got.Setup.ScaffoldDefaultStack {
		t.Fatalf("expected scaffolding to be disabled for external stack: %+v", got.Setup)
	}
	if got.Stack.Dir != externalDir || got.Stack.ComposeFile != "compose.custom.yaml" {
		t.Fatalf("unexpected external stack config: %+v", got.Stack)
	}
}

func TestRunWizardCanCustomizeServiceCredentials(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	input := strings.Join([]string{
		"", // stack name
		"", // managed stack
		"", // include postgres
		"", // postgres container
		"", // postgres image
		"", // postgres volume
		"", // postgres maintenance db
		"", // postgres port
		"stackdb",
		"stackuser",
		"stackpass",
		"", // include redis
		"", // redis container
		"", // redis image
		"", // redis volume
		"", // redis appendonly
		"", // redis save policy
		"", // redis maxmemory policy
		"", // redis port
		"redispass",
		"", // include nats
		"", // nats container
		"", // nats image
		"", // nats port
		"natssecret",
		"", // include seaweedfs
		"", // include pgadmin
		"", // pgadmin container
		"", // pgadmin image
		"", // pgadmin volume
		"", // pgadmin server mode
		"", // pgadmin port
		"pgadmin@example.com",
		"pgsecret",
		"", // include cockpit
		"", // cockpit port
		"", // install cockpit
		"", // wait for services
		"", // startup timeout
		"", // package manager
	}, "\n") + "\n"

	got, err := RunWizard(strings.NewReader(input), io.Discard, cfg)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}
	if got.Connection.PostgresDatabase != "stackdb" {
		t.Fatalf("unexpected postgres database: %q", got.Connection.PostgresDatabase)
	}
	if got.Connection.PostgresUsername != "stackuser" {
		t.Fatalf("unexpected postgres username: %q", got.Connection.PostgresUsername)
	}
	if got.Connection.PostgresPassword != "stackpass" {
		t.Fatalf("unexpected postgres password: %q", got.Connection.PostgresPassword)
	}
	if got.Connection.RedisPassword != "redispass" {
		t.Fatalf("unexpected redis password: %q", got.Connection.RedisPassword)
	}
	if got.Connection.NATSToken != "natssecret" {
		t.Fatalf("unexpected nats token: %q", got.Connection.NATSToken)
	}
	if got.Connection.PgAdminEmail != "pgadmin@example.com" {
		t.Fatalf("unexpected pgadmin email: %q", got.Connection.PgAdminEmail)
	}
	if got.Connection.PgAdminPassword != "pgsecret" {
		t.Fatalf("unexpected pgadmin password: %q", got.Connection.PgAdminPassword)
	}
}

func TestRunWizardCanCustomizeServiceRuntimeSettings(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	input := strings.Join([]string{
		"", // stack name
		"", // managed stack
		"", // include postgres
		"", // postgres container
		"docker.io/library/postgres:17",
		"stack_postgres_data",
		"template1",
		"", // postgres port
		"", // postgres database
		"", // postgres username
		"", // postgres password
		"", // include redis
		"", // redis container
		"docker.io/library/redis:7.4",
		"stack_redis_data",
		"y",
		"900 1 300 10",
		"allkeys-lru",
		"", // redis port
		"", // redis password
		"", // include nats
		"", // nats container
		"", // nats image
		"", // nats port
		"", // nats token
		"", // include seaweedfs
		"", // include pgadmin
		"", // pgadmin container
		"docker.io/dpage/pgadmin4:9",
		"stack_pgadmin_data",
		"y",
		"", // pgadmin port
		"", // pgadmin email
		"", // pgadmin password
		"", // include cockpit
		"", // cockpit port
		"", // install cockpit
		"", // wait for services
		"", // startup timeout
		"", // package manager
	}, "\n") + "\n"

	got, err := RunWizard(strings.NewReader(input), io.Discard, cfg)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}
	if got.Services.Postgres.Image != "docker.io/library/postgres:17" {
		t.Fatalf("unexpected postgres image: %q", got.Services.Postgres.Image)
	}
	if got.Services.Postgres.DataVolume != "stack_postgres_data" {
		t.Fatalf("unexpected postgres data volume: %q", got.Services.Postgres.DataVolume)
	}
	if got.Services.Postgres.MaintenanceDatabase != "template1" {
		t.Fatalf("unexpected postgres maintenance database: %q", got.Services.Postgres.MaintenanceDatabase)
	}
	if got.Services.Redis.Image != "docker.io/library/redis:7.4" {
		t.Fatalf("unexpected redis image: %q", got.Services.Redis.Image)
	}
	if got.Services.Redis.DataVolume != "stack_redis_data" {
		t.Fatalf("unexpected redis data volume: %q", got.Services.Redis.DataVolume)
	}
	if !got.Services.Redis.AppendOnly {
		t.Fatal("expected redis appendonly to be enabled")
	}
	if got.Services.Redis.SavePolicy != "900 1 300 10" {
		t.Fatalf("unexpected redis save policy: %q", got.Services.Redis.SavePolicy)
	}
	if got.Services.Redis.MaxMemoryPolicy != "allkeys-lru" {
		t.Fatalf("unexpected redis maxmemory policy: %q", got.Services.Redis.MaxMemoryPolicy)
	}
	if got.Services.PgAdmin.Image != "docker.io/dpage/pgadmin4:9" {
		t.Fatalf("unexpected pgadmin image: %q", got.Services.PgAdmin.Image)
	}
	if got.Services.PgAdmin.DataVolume != "stack_pgadmin_data" {
		t.Fatalf("unexpected pgadmin data volume: %q", got.Services.PgAdmin.DataVolume)
	}
	if !got.Services.PgAdmin.ServerMode {
		t.Fatal("expected pgadmin server mode to be enabled")
	}
}

func TestRunWizardCanEnableAndCustomizeSeaweedFS(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	input := strings.Join([]string{
		"", // stack name
		"", // managed stack
		"", // include postgres
		"", // postgres container
		"", // postgres image
		"", // postgres volume
		"", // postgres maintenance db
		"", // postgres port
		"", // postgres database
		"", // postgres username
		"", // postgres password
		"", // include redis
		"", // redis container
		"", // redis image
		"", // redis volume
		"", // redis appendonly
		"", // redis save policy
		"", // redis maxmemory policy
		"", // redis port
		"", // redis password
		"", // include nats
		"", // nats container
		"", // nats image
		"", // nats port
		"", // nats token
		"y",
		"", // seaweedfs container
		"", // seaweedfs image
		"stack_seaweedfs_data",
		"2048",
		"18333",
		"seaweed-access",
		"seaweed-secret",
		"", // include pgadmin
		"", // pgadmin container
		"", // pgadmin image
		"", // pgadmin volume
		"", // pgadmin server mode
		"", // pgadmin port
		"", // pgadmin email
		"", // pgadmin password
		"", // include cockpit
		"", // cockpit port
		"", // install cockpit
		"", // wait for services
		"", // startup timeout
		"", // package manager
	}, "\n") + "\n"

	got, err := RunWizard(strings.NewReader(input), io.Discard, cfg)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}
	if !got.Setup.IncludeSeaweedFS {
		t.Fatalf("expected seaweedfs to be enabled: %+v", got.Setup)
	}
	if got.Services.SeaweedFS.DataVolume != "stack_seaweedfs_data" {
		t.Fatalf("unexpected seaweedfs data volume: %q", got.Services.SeaweedFS.DataVolume)
	}
	if got.Services.SeaweedFS.VolumeSizeLimitMB != 2048 {
		t.Fatalf("unexpected seaweedfs volume size limit: %d", got.Services.SeaweedFS.VolumeSizeLimitMB)
	}
	if got.Ports.SeaweedFS != 18333 {
		t.Fatalf("unexpected seaweedfs port: %d", got.Ports.SeaweedFS)
	}
	if got.Connection.SeaweedFSAccessKey != "seaweed-access" {
		t.Fatalf("unexpected seaweedfs access key: %q", got.Connection.SeaweedFSAccessKey)
	}
	if got.Connection.SeaweedFSSecretKey != "seaweed-secret" {
		t.Fatalf("unexpected seaweedfs secret key: %q", got.Connection.SeaweedFSSecretKey)
	}
}

func TestRunWizardRejectsDisablingEveryStackService(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	var out strings.Builder
	input := strings.Join([]string{
		"",
		"",
		"n",
		"n",
		"n",
		"n",
		"n",
	}, "\n") + "\n"

	_, err := RunWizard(strings.NewReader(input), &out, cfg)
	if err == nil || !strings.Contains(err.Error(), "at least one stack service must be enabled") {
		t.Fatalf("unexpected wizard error: %v", err)
	}
	if strings.Contains(out.String(), "Include Cockpit helpers") {
		t.Fatalf("wizard should stop before cockpit when every stack service is disabled:\n%s", out.String())
	}
}

func TestPromptYesNoAndValidationHelpers(t *testing.T) {
	yes, err := PromptYesNo(strings.NewReader("y\n"), io.Discard, "Continue?", false)
	if err != nil {
		t.Fatalf("PromptYesNo returned error: %v", err)
	}
	if !yes {
		t.Fatal("expected yes response")
	}

	if err := nonEmpty(""); err == nil {
		t.Fatal("expected nonEmpty to fail")
	}
	if err := positiveInt(0); err == nil {
		t.Fatal("expected positiveInt to fail")
	}
	if err := validPort(70000); err == nil {
		t.Fatal("expected validPort to fail")
	}
}

func TestPromptSessionFormatsBooleanDefaultsConsistently(t *testing.T) {
	var out strings.Builder

	session := promptSession{
		reader: bufio.NewReader(strings.NewReader("\n\n")),
		out:    &out,
	}

	if _, err := session.askBool("Proceed", true); err != nil {
		t.Fatalf("askBool returned error: %v", err)
	}
	if _, err := session.askBool("Proceed", false); err != nil {
		t.Fatalf("askBool returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Proceed [Y/n]: ") {
		t.Fatalf("missing [Y/n] prompt: %q", got)
	}
	if !strings.Contains(got, "Proceed [y/N]: ") {
		t.Fatalf("missing [y/N] prompt: %q", got)
	}
}

func TestValidationErrorString(t *testing.T) {
	err := ValidationError{
		Issues: []ValidationIssue{{Field: "stack.dir", Message: "missing"}},
	}

	if !strings.Contains(err.Error(), "stack.dir: missing") {
		t.Fatalf("unexpected validation error string: %s", err.Error())
	}
}

func TestValidationErrorStringWithoutIssues(t *testing.T) {
	if got := (ValidationError{}).Error(); got != "config validation failed" {
		t.Fatalf("unexpected validation error string: %s", got)
	}
}

func TestPromptSessionRetriesInvalidAnswers(t *testing.T) {
	session := promptSession{
		reader: bufio.NewReader(strings.NewReader("\ncustom\nabc\n42\nmaybe\ny\n\ny\n")),
		out:    io.Discard,
	}

	value, err := session.askString("Name", "", nonEmpty)
	if err != nil || value != "custom" {
		t.Fatalf("askString returned %q, %v", value, err)
	}

	number, err := session.askInt("Timeout", 30, positiveInt)
	if err != nil || number != 42 {
		t.Fatalf("askInt returned %d, %v", number, err)
	}

	boolean, err := session.askBool("Open", false)
	if err != nil || !boolean {
		t.Fatalf("askBool returned %v, %v", boolean, err)
	}

	dir, err := session.askStackDir(filepath.Join(t.TempDir(), "missing"))
	if err != nil || dir == "" {
		t.Fatalf("askStackDir returned %q, %v", dir, err)
	}
}

func TestPromptSessionAcceptsDefaultAnswers(t *testing.T) {
	session := promptSession{
		reader: bufio.NewReader(strings.NewReader("\n\n\n")),
		out:    io.Discard,
	}

	value, err := session.askString("Name", "default", nonEmpty)
	if err != nil || value != "default" {
		t.Fatalf("askString default returned %q, %v", value, err)
	}

	number, err := session.askInt("Timeout", 30, positiveInt)
	if err != nil || number != 30 {
		t.Fatalf("askInt default returned %d, %v", number, err)
	}

	boolean, err := session.askBool("Open", true)
	if err != nil || !boolean {
		t.Fatalf("askBool default returned %v, %v", boolean, err)
	}
}

func TestPromptSessionInvalidBooleanAtEOFReturnsError(t *testing.T) {
	session := promptSession{
		reader: bufio.NewReader(strings.NewReader("maybe")),
		out:    io.Discard,
	}

	_, err := session.askBool("Open", false)
	if err == nil || !strings.Contains(err.Error(), "invalid boolean answer") {
		t.Fatalf("unexpected bool error: %v", err)
	}
}

func TestShouldUsePlainWizardForNonTerminalIO(t *testing.T) {
	if !shouldUsePlainWizard(strings.NewReader(""), io.Discard) {
		t.Fatal("expected non-terminal IO to use plain wizard prompts")
	}
}

func TestShouldUsePlainWizardHonorsOverrideEnv(t *testing.T) {
	t.Setenv("STACKCTL_WIZARD_PLAIN", "1")

	if !shouldUsePlainWizard(os.Stdin, os.Stdout) {
		t.Fatal("expected override env to force the plain wizard")
	}
}

func TestShouldUsePlainWizardDoesNotForceLegacyFlowForAccessibleTerminals(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open returned error: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	if shouldUsePlainWizard(tty, tty) {
		t.Fatal("expected accessible terminals to keep the Huh wizard flow")
	}
}

func TestNewWizardStateIncludesEnabledServices(t *testing.T) {
	cfg := Default()

	state := newWizardState(cfg)

	for _, service := range []string{"postgres", "redis", "nats", "pgadmin"} {
		if !state.includesService(service) {
			t.Fatalf("expected service %q to be selected by default", service)
		}
	}
	if !state.IncludeCockpit || !state.InstallCockpit {
		t.Fatalf("expected cockpit helper defaults, got %+v", state)
	}
}

func TestWizardStateToConfigAppliesManagedSelections(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	base := Default()
	state := newWizardState(base)
	state.StackName = "staging"
	state.StackMode = wizardStackModeManaged
	state.Services = []string{"postgres", "nats"}
	state.IncludeCockpit = false
	state.InstallCockpit = false
	state.PostgresPort = "15432"
	state.NATSPort = "14222"
	state.StartupTimeoutSec = "45"
	state.PackageManager = "dnf"

	cfg, err := state.toConfig(base)
	if err != nil {
		t.Fatalf("toConfig returned error: %v", err)
	}

	wantDir, err := ManagedStackDir("staging")
	if err != nil {
		t.Fatalf("ManagedStackDir returned error: %v", err)
	}
	if !cfg.Stack.Managed || cfg.Stack.Dir != wantDir || cfg.Stack.ComposeFile != DefaultComposeFileName {
		t.Fatalf("unexpected managed stack config: %+v", cfg.Stack)
	}
	if !cfg.Setup.IncludePostgres || cfg.Setup.IncludeRedis || !cfg.Setup.IncludeNATS || cfg.Setup.IncludePgAdmin {
		t.Fatalf("unexpected service toggles: %+v", cfg.Setup)
	}
	if cfg.Setup.IncludeCockpit || cfg.Setup.InstallCockpit {
		t.Fatalf("unexpected cockpit toggles: %+v", cfg.Setup)
	}
	if cfg.Ports.Postgres != 15432 || cfg.Ports.NATS != 14222 {
		t.Fatalf("unexpected managed service ports: %+v", cfg.Ports)
	}
	if cfg.Behavior.StartupTimeoutSec != 45 || cfg.System.PackageManager != "dnf" {
		t.Fatalf("unexpected behavior/system config: %+v %+v", cfg.Behavior, cfg.System)
	}
}

func TestWizardStateToConfigAppliesExternalSettings(t *testing.T) {
	base := Default()
	externalDir := filepath.Join(t.TempDir(), "external")
	state := newWizardState(base)
	state.StackMode = wizardStackModeExternal
	state.ExternalStackDir = externalDir
	state.ExternalComposeFile = "compose.custom.yaml"

	cfg, err := state.toConfig(base)
	if err != nil {
		t.Fatalf("toConfig returned error: %v", err)
	}

	if cfg.Stack.Managed || cfg.Setup.ScaffoldDefaultStack {
		t.Fatalf("expected external stack config, got %+v / %+v", cfg.Stack, cfg.Setup)
	}
	if cfg.Stack.Dir != externalDir || cfg.Stack.ComposeFile != "compose.custom.yaml" {
		t.Fatalf("unexpected external stack config: %+v", cfg.Stack)
	}
}

func TestWizardReviewSummaryIncludesCurrentSelections(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	state := newWizardState(Default())
	state.StackName = "staging"
	state.StackMode = wizardStackModeManaged
	state.Services = []string{"postgres", "nats"}
	state.IncludeCockpit = false
	state.PostgresPort = "15432"
	state.NATSPort = "14222"
	state.StartupTimeoutSec = "45"
	state.PackageManager = "dnf"

	summary := state.reviewSummary()
	for _, fragment := range []string{
		"Stack: staging",
		"Mode: Managed",
		"Services: Postgres, NATS",
		"Postgres: 15432",
		"NATS: 14222",
		"Package manager: dnf",
	} {
		if !strings.Contains(summary, fragment) {
			t.Fatalf("review summary missing %q:\n%s", fragment, summary)
		}
	}
}

func TestWizardStateMissingExternalDirConfirmation(t *testing.T) {
	state := wizardState{
		StackMode:        wizardStackModeExternal,
		ExternalStackDir: filepath.Join(t.TempDir(), "missing"),
	}
	if !state.needsMissingExternalDirConfirmation() {
		t.Fatal("expected missing external dir to require confirmation")
	}

	existingDir := t.TempDir()
	state.ExternalStackDir = existingDir
	if state.needsMissingExternalDirConfirmation() {
		t.Fatal("did not expect an existing external dir to require confirmation")
	}
}

func TestWizardStepPositionTracksVisiblePages(t *testing.T) {
	state := newWizardState(Default())
	if position, total := wizardStepPosition(&state, wizardStepStack); position != 1 || total != 11 {
		t.Fatalf("unexpected managed stack step position: %d/%d", position, total)
	}
	if position, total := wizardStepPosition(&state, wizardStepReview); position != 11 || total != 11 {
		t.Fatalf("unexpected managed review step position: %d/%d", position, total)
	}

	state.StackMode = wizardStackModeExternal
	state.ExternalStackDir = filepath.Join(t.TempDir(), "missing")
	state.IncludeCockpit = true
	state.Services = []string{"postgres", "redis", "nats", "seaweedfs", "pgadmin"}
	if position, total := wizardStepPosition(&state, wizardStepExternalPath); position != 3 || total != 14 {
		t.Fatalf("unexpected external path step position: %d/%d", position, total)
	}
	if next := wizardNextStepLabel(&state, wizardStepServices); next != "Postgres settings" {
		t.Fatalf("unexpected next step label: %q", next)
	}
}

func TestRunWizardPropagatesPromptReadErrors(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()

	cases := []struct {
		name             string
		completedPrompts int
	}{
		{name: "stack name", completedPrompts: 0},
		{name: "manage stack", completedPrompts: 1},
		{name: "include postgres", completedPrompts: 2},
		{name: "postgres container", completedPrompts: 3},
		{name: "postgres image", completedPrompts: 4},
		{name: "postgres volume", completedPrompts: 5},
		{name: "postgres maintenance db", completedPrompts: 6},
		{name: "postgres port", completedPrompts: 7},
		{name: "postgres database", completedPrompts: 8},
		{name: "postgres username", completedPrompts: 9},
		{name: "postgres password", completedPrompts: 10},
		{name: "include redis", completedPrompts: 11},
		{name: "redis container", completedPrompts: 12},
		{name: "redis image", completedPrompts: 13},
		{name: "redis volume", completedPrompts: 14},
		{name: "redis appendonly", completedPrompts: 15},
		{name: "redis save policy", completedPrompts: 16},
		{name: "redis maxmemory policy", completedPrompts: 17},
		{name: "redis port", completedPrompts: 18},
		{name: "redis password", completedPrompts: 19},
		{name: "include nats", completedPrompts: 20},
		{name: "nats container", completedPrompts: 21},
		{name: "nats image", completedPrompts: 22},
		{name: "nats port", completedPrompts: 23},
		{name: "nats token", completedPrompts: 24},
		{name: "include pgadmin", completedPrompts: 25},
		{name: "pgadmin container", completedPrompts: 26},
		{name: "pgadmin image", completedPrompts: 27},
		{name: "pgadmin volume", completedPrompts: 28},
		{name: "pgadmin server mode", completedPrompts: 29},
		{name: "pgadmin port", completedPrompts: 30},
		{name: "pgadmin email", completedPrompts: 31},
		{name: "pgadmin password", completedPrompts: 32},
		{name: "include cockpit", completedPrompts: 33},
		{name: "cockpit port", completedPrompts: 34},
		{name: "install cockpit", completedPrompts: 35},
		{name: "wait for services", completedPrompts: 36},
		{name: "timeout", completedPrompts: 37},
		{name: "package manager", completedPrompts: 38},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunWizard(&failingLinesReader{
				remaining: tc.completedPrompts,
				err:       io.ErrUnexpectedEOF,
			}, io.Discard, cfg)
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("expected read error, got %v", err)
			}
		})
	}
}

func TestManagedStackDirUsesXDGDataHome(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataRoot)

	got, err := ManagedStackDir(DefaultStackName)
	if err != nil {
		t.Fatalf("ManagedStackDir returned error: %v", err)
	}

	want := filepath.Join(dataRoot, "stackctl", "stacks", DefaultStackName)
	if got != want {
		t.Fatalf("unexpected managed stack dir: %s", got)
	}
	if DefaultManagedStackDir() != want {
		t.Fatalf("unexpected default managed stack dir: %s", DefaultManagedStackDir())
	}
}

func TestDataDirFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", home)

	got, err := DataDirPath()
	if err != nil {
		t.Fatalf("DataDirPath returned error: %v", err)
	}

	want := filepath.Join(home, ".local", "share", "stackctl")
	if got != want {
		t.Fatalf("unexpected data dir: %s", got)
	}
}

func TestConfigPathsFailWithoutUserConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	if _, err := ConfigDirPath(); err == nil {
		t.Fatal("expected ConfigDirPath to fail without a user config dir")
	}
	if _, err := ConfigFilePath(); err == nil {
		t.Fatal("expected ConfigFilePath to fail without a user config dir")
	}
	if _, err := DataDirPath(); err == nil {
		t.Fatal("expected DataDirPath to fail without a user home dir")
	}
}

type failingLinesReader struct {
	remaining int
	err       error
}

func (r *failingLinesReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		return 0, r.err
	}

	p[0] = '\n'
	r.remaining--
	return 1, nil
}
