package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldManagedStackAlreadyPresentAndForceRewrite(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()

	first, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("initial ScaffoldManagedStack returned error: %v", err)
	}
	if !first.WroteCompose {
		t.Fatalf("expected initial scaffold to write compose data, got %+v", first)
	}

	second, err := ScaffoldManagedStack(cfg, false)
	if err != nil {
		t.Fatalf("second ScaffoldManagedStack returned error: %v", err)
	}
	if !second.AlreadyPresent || second.WroteCompose || second.WroteNATSConfig || second.WrotePgAdminServers || second.WrotePGPass {
		t.Fatalf("expected already-present scaffold result, got %+v", second)
	}

	forced, err := ScaffoldManagedStack(cfg, true)
	if err != nil {
		t.Fatalf("forced ScaffoldManagedStack returned error: %v", err)
	}
	if !forced.WroteCompose || !forced.WroteNATSConfig || !forced.WrotePgAdminServers || !forced.WrotePGPass {
		t.Fatalf("expected force scaffold to rewrite managed files, got %+v", forced)
	}
	if forced.AlreadyPresent {
		t.Fatalf("expected forced scaffold to report writes, got %+v", forced)
	}
}

func TestScaffoldManagedStackRejectsInvalidManagedConfigurations(t *testing.T) {
	t.Run("unmanaged stack", func(t *testing.T) {
		cfg := Default()
		cfg.Stack.Managed = false

		_, err := ScaffoldManagedStack(cfg, false)
		if err == nil || !strings.Contains(err.Error(), "stack.managed = true") {
			t.Fatalf("unexpected unmanaged scaffold error: %v", err)
		}
	})

	t.Run("unexpected managed dir", func(t *testing.T) {
		cfg := Default()
		cfg.Stack.Dir = filepath.Join(t.TempDir(), "elsewhere")

		_, err := ScaffoldManagedStack(cfg, false)
		if err == nil || !strings.Contains(err.Error(), "managed stack dir must be") {
			t.Fatalf("unexpected managed-dir error: %v", err)
		}
	})

	t.Run("unexpected compose file", func(t *testing.T) {
		cfg := Default()
		cfg.Stack.ComposeFile = "custom-compose.yaml"

		_, err := ScaffoldManagedStack(cfg, false)
		if err == nil || !strings.Contains(err.Error(), "managed stack compose file must be") {
			t.Fatalf("unexpected managed-compose error: %v", err)
		}
	})

	t.Run("stack dir path is a file", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", t.TempDir())

		cfg := Default()
		if err := os.MkdirAll(filepath.Dir(cfg.Stack.Dir), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		if err := os.WriteFile(cfg.Stack.Dir, []byte("not-a-directory"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		_, err := ScaffoldManagedStack(cfg, false)
		if err == nil || !strings.Contains(err.Error(), "is not a directory") {
			t.Fatalf("unexpected stack-dir-file error: %v", err)
		}
	})
}

func TestScaffoldFileHelpersCoverMissingAndDirectoryCases(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "compose.yaml")
	payload := []byte("services: {}\n")

	if missing, err := scaffoldFileMissing(target); err != nil || !missing {
		t.Fatalf("expected missing scaffold file, got missing=%v err=%v", missing, err)
	}
	if needsWrite, err := scaffoldFileNeedsWrite(target, payload); err != nil || !needsWrite {
		t.Fatalf("expected missing scaffold file to need a write, got needsWrite=%v err=%v", needsWrite, err)
	}

	wrote, err := writeScaffoldFile(target, payload, false)
	if err != nil {
		t.Fatalf("writeScaffoldFile returned error: %v", err)
	}
	if !wrote {
		t.Fatal("expected first scaffold write to report a write")
	}

	wrote, err = writeScaffoldFile(target, payload, false)
	if err != nil {
		t.Fatalf("writeScaffoldFile returned error on identical payload: %v", err)
	}
	if wrote {
		t.Fatal("expected identical scaffold payload to skip rewriting")
	}

	wrote, err = writeScaffoldFile(target, payload, true)
	if err != nil {
		t.Fatalf("writeScaffoldFile force returned error: %v", err)
	}
	if !wrote {
		t.Fatal("expected forced scaffold write to rewrite the file")
	}

	if missing, err := scaffoldFileMissing(dir); err == nil || missing || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected directory path to report an is-a-directory error, got missing=%v err=%v", missing, err)
	}
	if _, err := scaffoldFileNeedsWrite(dir, payload); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("unexpected scaffoldFileNeedsWrite directory error: %v", err)
	}
}

func TestManagedStackNeedsScaffoldDetectsMissingGeneratedFiles(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Connection.RedisACLUsername = "stackctl"
	cfg.Connection.RedisACLPassword = "redis-acl-secret"

	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}
	if err := os.Remove(RedisACLPath(cfg)); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if !needsScaffold {
		t.Fatal("expected missing generated redis ACL file to require scaffolding")
	}
}

func TestScaffoldFileMissingTreatsNotExistAsMissing(t *testing.T) {
	missing, err := scaffoldFileMissing(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("scaffoldFileMissing returned error: %v", err)
	}
	if !missing {
		t.Fatal("expected missing file to report true")
	}

	target := filepath.Join(t.TempDir(), "present.yaml")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	missing, err = scaffoldFileMissing(target)
	if err != nil {
		t.Fatalf("scaffoldFileMissing returned error: %v", err)
	}
	if missing {
		t.Fatal("expected present file to report false")
	}
}

func TestWriteScaffoldFilePropagatesCreateErrors(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parent, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := writeScaffoldFile(filepath.Join(parent, "compose.yaml"), []byte("services: {}\n"), false)
	if err == nil || (!strings.Contains(err.Error(), "not a directory") && !errors.Is(err, os.ErrNotExist)) {
		t.Fatalf("unexpected writeScaffoldFile error: %v", err)
	}
}
