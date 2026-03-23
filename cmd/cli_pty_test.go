package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/testutil"
)

func TestConfigInitInteractivePTYCustomizesConfig(t *testing.T) {
	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("HOME", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	env := cliTestEnv(t, configRoot, dataRoot)

	input := strings.Join([]string{
		"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
		"15432",
		"16379",
		"18081",
		"19090",
		"stackdb",
		"stackuser",
		"stackpass",
		"redispass",
		"pgadmin@example.com",
		"pgsecret",
		"", "", "", "",
	}, "\n") + "\n"

	output, err := runStackctlPTY(t, binaryPath, env, input, "config", "init")
	if err != nil {
		t.Fatalf("config init returned error: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Postgres database name") || !strings.Contains(output, "pgAdmin password") {
		t.Fatalf("expected wizard prompts in tty output, got:\n%s", output)
	}

	configPath := filepath.Join(configRoot, "stackctl", "config.yaml")
	cfg, err := configpkg.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Connection.PostgresDatabase != "stackdb" || cfg.Connection.PostgresUsername != "stackuser" || cfg.Connection.PostgresPassword != "stackpass" {
		t.Fatalf("unexpected postgres config: %+v", cfg.Connection)
	}
	if cfg.Connection.RedisPassword != "redispass" {
		t.Fatalf("unexpected redis password: %+v", cfg.Connection)
	}
	if cfg.Connection.PgAdminEmail != "pgadmin@example.com" || cfg.Connection.PgAdminPassword != "pgsecret" {
		t.Fatalf("unexpected pgadmin config: %+v", cfg.Connection)
	}

	composeData, err := os.ReadFile(filepath.Join(dataRoot, "stackctl", "stacks", "dev-stack", "compose.yaml"))
	if err != nil {
		t.Fatalf("read scaffolded compose file: %v", err)
	}
	composeText := string(composeData)
	for _, fragment := range []string{
		"POSTGRES_USER: \"stackuser\"",
		"POSTGRES_PASSWORD: \"stackpass\"",
		"POSTGRES_DB: \"stackdb\"",
		"--requirepass",
		"\"redispass\"",
		"PGADMIN_DEFAULT_EMAIL: \"pgadmin@example.com\"",
		"PGADMIN_DEFAULT_PASSWORD: \"pgsecret\"",
	} {
		if !strings.Contains(composeText, fragment) {
			t.Fatalf("compose file missing %q:\n%s", fragment, composeText)
		}
	}
}

func TestConfigEditInteractivePTYUpdatesExistingConfig(t *testing.T) {
	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("HOME", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	env := cliTestEnv(t, configRoot, dataRoot)

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()
	configPath := filepath.Join(configRoot, "stackctl", "config.yaml")
	if err := configpkg.Save(configPath, cfg); err != nil {
		t.Fatalf("save initial config: %v", err)
	}
	if _, err := configpkg.ScaffoldManagedStack(cfg, true); err != nil {
		t.Fatalf("scaffold initial managed stack: %v", err)
	}

	input := strings.Join([]string{
		"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
		"25432",
		"26379",
		"28081",
		"29090",
		"editeddb",
		"editeduser",
		"editedpass",
		"",
		"ops@example.com",
		"opspass",
		"", "", "", "",
	}, "\n") + "\n"

	output, err := runStackctlPTY(t, binaryPath, env, input, "config", "edit")
	if err != nil {
		t.Fatalf("config edit returned error: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Updated config at") {
		t.Fatalf("unexpected config edit output:\n%s", output)
	}

	updated, err := configpkg.Load(configPath)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	if updated.Ports.Postgres != 25432 || updated.Ports.Redis != 26379 || updated.Ports.PgAdmin != 28081 || updated.Ports.Cockpit != 29090 {
		t.Fatalf("unexpected updated ports: %+v", updated.Ports)
	}
	if updated.Connection.PostgresDatabase != "editeddb" || updated.Connection.PostgresUsername != "editeduser" || updated.Connection.PostgresPassword != "editedpass" {
		t.Fatalf("unexpected updated postgres config: %+v", updated.Connection)
	}
	if updated.Connection.PgAdminEmail != "ops@example.com" || updated.Connection.PgAdminPassword != "opspass" {
		t.Fatalf("unexpected updated pgadmin config: %+v", updated.Connection)
	}

	connectOutput, err := runStackctl(t, binaryPath, env, "connect")
	if err != nil {
		t.Fatalf("connect returned error: %v\n%s", err, connectOutput)
	}
	for _, fragment := range []string{
		"postgres://editeduser:editedpass@localhost:25432/editeddb",
		"redis://localhost:26379",
		"http://localhost:28081",
		"https://localhost:29090",
	} {
		if !strings.Contains(connectOutput, fragment) {
			t.Fatalf("connect output missing %q:\n%s", fragment, connectOutput)
		}
	}
}

func cliTestEnv(t testing.TB, configRoot, dataRoot string) []string {
	t.Helper()

	fakeBin := t.TempDir()
	podmanPath := filepath.Join(fakeBin, "podman")
	podmanScript := `#!/bin/sh
case "$1" in
  compose)
    shift
    while [ "$#" -gt 0 ]; do
      case "$1" in
        -f)
          shift 2
          ;;
        version)
          exit 0
          ;;
        ps)
          printf '[]\n'
          exit 0
          ;;
        *)
          shift
          ;;
      esac
    done
    exit 0
    ;;
  ps)
    printf '[]\n'
    exit 0
    ;;
  container)
    if [ "$2" = "exists" ]; then
      exit 1
    fi
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(podmanPath, []byte(podmanScript), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}

	return []string{
		"HOME=" + dataRoot,
		"XDG_CONFIG_HOME=" + configRoot,
		"XDG_DATA_HOME=" + dataRoot,
		"PATH=" + fakeBin,
	}
}

func runStackctl(t testing.TB, binaryPath string, env []string, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = testutil.RepoRoot()
	cmd.Env = testutil.MergeEnv(env)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("stackctl command timed out: %v", args)
	}

	return string(output), err
}

func runStackctlPTY(t testing.TB, binaryPath string, env []string, input string, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = testutil.RepoRoot()
	cmd.Env = testutil.MergeEnv(env)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("start pty command: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	go func() {
		if input != "" {
			_, _ = io.Copy(ptmx, strings.NewReader(input))
		}
	}()

	var output bytes.Buffer
	_, readErr := io.Copy(&output, ptmx)
	waitErr := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("stackctl pty command timed out: %v", args)
	}
	if readErr != nil && !strings.Contains(readErr.Error(), "input/output error") {
		t.Fatalf("read pty output: %v", readErr)
	}

	return output.String(), waitErr
}
