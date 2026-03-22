package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedStackNeedsScaffoldTracksComposePresence(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if !needsScaffold {
		t.Fatal("expected managed stack to need scaffolding before compose exists")
	}

	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	needsScaffold, err = ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if needsScaffold {
		t.Fatal("expected managed stack to be fully scaffolded")
	}
}

func TestManagedStackNeedsScaffoldIgnoresExternalStack(t *testing.T) {
	cfg := Default()
	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if needsScaffold {
		t.Fatal("external stack should not request managed scaffolding")
	}
}

func TestManagedStackNeedsScaffoldErrorsWhenComposePathIsDirectory(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if err := os.MkdirAll(ComposePath(cfg), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	_, err := ManagedStackNeedsScaffold(cfg)
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScaffoldManagedStackCreatesComposeFile(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Connection.PostgresUsername = "stackuser"
	cfg.Connection.PostgresPassword = "stackpass"
	cfg.Connection.PostgresDatabase = "stackdb"
	cfg.Ports.Postgres = 15432

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}
	if !result.CreatedDir || !result.WroteCompose {
		t.Fatalf("unexpected scaffold result: %+v", result)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if !strings.Contains(string(data), "local-postgres") {
		t.Fatalf("unexpected scaffolded compose file: %s", string(data))
	}
	if !strings.Contains(string(data), "POSTGRES_USER: \"stackuser\"") {
		t.Fatalf("expected rendered postgres username, got: %s", string(data))
	}
	if !strings.Contains(string(data), "POSTGRES_PASSWORD: \"stackpass\"") {
		t.Fatalf("expected rendered postgres password, got: %s", string(data))
	}
	if !strings.Contains(string(data), "POSTGRES_DB: \"stackdb\"") {
		t.Fatalf("expected rendered postgres database, got: %s", string(data))
	}
	if !strings.Contains(string(data), "\"15432:5432\"") {
		t.Fatalf("expected rendered postgres port mapping, got: %s", string(data))
	}
}

func TestScaffoldManagedStackOmitsPgAdminWhenDisabled(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Setup.IncludePgAdmin = false

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	data, err := os.ReadFile(result.ComposePath)
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	if strings.Contains(string(data), "pgadmin:") {
		t.Fatalf("expected pgadmin service to be omitted, got: %s", string(data))
	}
}

func TestScaffoldManagedStackTreatsExistingComposeAsAlreadyPresent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("initial scaffold failed: %v", err)
	}

	result, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("repeat scaffold failed: %v", err)
	}
	if !result.AlreadyPresent || result.WroteCompose {
		t.Fatalf("unexpected scaffold result: %+v", result)
	}
}

func TestScaffoldManagedStackOverwritesWhenForced(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("initial scaffold failed: %v", err)
	}
	if err := os.WriteFile(ComposePath(cfg), []byte("custom: true\n"), 0o644); err != nil {
		t.Fatalf("write custom compose file failed: %v", err)
	}

	result, err := ScaffoldManagedStack(cfg, true)
	if err != nil {
		t.Fatalf("forced scaffold failed: %v", err)
	}
	if !result.WroteCompose {
		t.Fatalf("expected forced scaffold to rewrite compose file: %+v", result)
	}

	data, err := os.ReadFile(ComposePath(cfg))
	if err != nil {
		t.Fatalf("read compose file failed: %v", err)
	}
	if strings.Contains(string(data), "custom: true") {
		t.Fatalf("expected embedded template overwrite, got %s", string(data))
	}
}

func TestScaffoldManagedStackRejectsExternalOrInconsistentPaths(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Stack.Managed = false
	if _, err := ScaffoldManagedStack(cfg, false); err == nil || !strings.Contains(err.Error(), "stack.managed") {
		t.Fatalf("unexpected external stack scaffold error: %v", err)
	}

	cfg = Default()
	cfg.Stack.Dir = filepath.Join(t.TempDir(), "other")
	if _, err := ScaffoldManagedStack(cfg, false); err == nil || !strings.Contains(err.Error(), "managed stack dir must be") {
		t.Fatalf("unexpected mismatched managed path error: %v", err)
	}
}

func TestScaffoldManagedStackRejectsFileAtStackPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if err := os.MkdirAll(filepath.Dir(cfg.Stack.Dir), 0o755); err != nil {
		t.Fatalf("mkdir parent failed: %v", err)
	}
	if err := os.WriteFile(cfg.Stack.Dir, []byte("file"), 0o644); err != nil {
		t.Fatalf("write stack path file failed: %v", err)
	}
	if _, err := ScaffoldManagedStack(cfg, false); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("unexpected stack path error: %v", err)
	}
}

func TestScaffoldManagedStackRejectsDirectoryAtComposePath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if err := os.MkdirAll(ComposePath(cfg), 0o755); err != nil {
		t.Fatalf("mkdir compose path failed: %v", err)
	}
	if _, err := ScaffoldManagedStack(cfg, false); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("unexpected compose path error: %v", err)
	}
}

func TestDefaultManagedStackDirReturnsEmptyWhenDataPathFails(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")

	if got := DefaultManagedStackDir(); got != "" {
		t.Fatalf("expected empty managed stack dir, got %s", got)
	}
}

func TestManagedStacksDirPathAndManagedStackDirUseDefaultName(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataRoot)

	stacksDir, err := ManagedStacksDirPath()
	if err != nil {
		t.Fatalf("ManagedStacksDirPath returned error: %v", err)
	}
	if stacksDir != filepath.Join(dataRoot, "stackctl", "stacks") {
		t.Fatalf("unexpected stacks dir: %s", stacksDir)
	}

	stackDir, err := ManagedStackDir("")
	if err != nil {
		t.Fatalf("ManagedStackDir returned error: %v", err)
	}
	if stackDir != filepath.Join(stacksDir, DefaultStackName) {
		t.Fatalf("unexpected managed stack dir: %s", stackDir)
	}
}

func TestDataDirPathFailsWithoutHomeOrXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")

	_, err := DataDirPath()
	if err == nil {
		t.Fatal("expected DataDirPath to fail")
	}
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "user home directory") {
		t.Fatalf("unexpected data dir error: %v", err)
	}
}
