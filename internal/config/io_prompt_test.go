package config

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadAndMarshalRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Default()

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

	input := strings.Repeat("\n", 20)
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

func TestRunWizardCanSwitchToExternalStack(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	externalDir := filepath.Join(t.TempDir(), "external-stack")
	if err := os.MkdirAll(externalDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	cfg := Default()
	input := "dev-stack\nn\n" + externalDir + "\ncompose.custom.yaml\n" + strings.Repeat("\n", 18)

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
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"stackdb",
		"stackuser",
		"stackpass",
		"redispass",
		"pgadmin@example.com",
		"pgsecret",
		"",
		"",
		"",
		"",
		"",
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
	if got.Connection.PgAdminEmail != "pgadmin@example.com" {
		t.Fatalf("unexpected pgadmin email: %q", got.Connection.PgAdminEmail)
	}
	if got.Connection.PgAdminPassword != "pgsecret" {
		t.Fatalf("unexpected pgadmin password: %q", got.Connection.PgAdminPassword)
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

func TestRunWizardPropagatesPromptReadErrors(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()

	cases := []struct {
		name             string
		completedPrompts int
	}{
		{name: "stack name", completedPrompts: 0},
		{name: "manage stack", completedPrompts: 1},
		{name: "postgres container", completedPrompts: 2},
		{name: "redis container", completedPrompts: 3},
		{name: "pgadmin container", completedPrompts: 4},
		{name: "postgres port", completedPrompts: 5},
		{name: "redis port", completedPrompts: 6},
		{name: "pgadmin port", completedPrompts: 7},
		{name: "cockpit port", completedPrompts: 8},
		{name: "postgres database", completedPrompts: 9},
		{name: "postgres username", completedPrompts: 10},
		{name: "postgres password", completedPrompts: 11},
		{name: "redis password", completedPrompts: 12},
		{name: "pgadmin email", completedPrompts: 13},
		{name: "pgadmin password", completedPrompts: 14},
		{name: "wait for services", completedPrompts: 15},
		{name: "timeout", completedPrompts: 16},
		{name: "install cockpit", completedPrompts: 17},
		{name: "include pgadmin", completedPrompts: 18},
		{name: "package manager", completedPrompts: 19},
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
