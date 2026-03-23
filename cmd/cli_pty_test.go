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
		"", // stack name
		"", // managed stack
		"", // include postgres
		"", // postgres container
		"", // postgres image
		"", // postgres volume
		"", // postgres maintenance db
		"15432",
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
		"16379",
		"redispass",
		"", // include nats
		"", // nats container
		"", // nats image
		"14222",
		"natssecret",
		"", // include pgadmin
		"", // pgadmin container
		"", // pgadmin image
		"", // pgadmin volume
		"", // pgadmin server mode
		"18081",
		"pgadmin@example.com",
		"pgsecret",
		"", // include cockpit
		"19090",
		"", // install cockpit
		"", // wait for services
		"", // startup timeout
		"", // package manager
	}, "\n") + "\n"

	output, err := runStackctlPTY(t, binaryPath, env, input, "config", "init")
	if err != nil {
		t.Fatalf("config init returned error: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Include Postgres in the stack") || !strings.Contains(output, "pgAdmin password") {
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
	if cfg.Connection.NATSToken != "natssecret" || cfg.Ports.NATS != 14222 {
		t.Fatalf("unexpected nats config: %+v / %+v", cfg.Connection, cfg.Ports)
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

	natsConfig, err := os.ReadFile(filepath.Join(dataRoot, "stackctl", "stacks", "dev-stack", "nats.conf"))
	if err != nil {
		t.Fatalf("read scaffolded nats config: %v", err)
	}
	if !strings.Contains(string(natsConfig), "token: \"natssecret\"") {
		t.Fatalf("nats config missing token:\n%s", string(natsConfig))
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
		"", // stack name
		"", // managed stack
		"", // include postgres
		"", // postgres container
		"", // postgres image
		"", // postgres volume
		"", // postgres maintenance db
		"25432",
		"editeddb",
		"editeduser",
		"editedpass",
		"", // include redis
		"", // redis container
		"", // redis image
		"", // redis volume
		"", // redis appendonly
		"", // redis save policy
		"", // redis maxmemory policy
		"26379",
		"",
		"", // include nats
		"", // nats container
		"", // nats image
		"24222",
		"editedtoken",
		"", // include pgadmin
		"", // pgadmin container
		"", // pgadmin image
		"", // pgadmin volume
		"", // pgadmin server mode
		"28081",
		"ops@example.com",
		"opspass",
		"", // include cockpit
		"29090",
		"", // install cockpit
		"", // wait for services
		"", // startup timeout
		"", // package manager
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
	if updated.Ports.Postgres != 25432 || updated.Ports.Redis != 26379 || updated.Ports.NATS != 24222 || updated.Ports.PgAdmin != 28081 || updated.Ports.Cockpit != 29090 {
		t.Fatalf("unexpected updated ports: %+v", updated.Ports)
	}
	if updated.Connection.PostgresDatabase != "editeddb" || updated.Connection.PostgresUsername != "editeduser" || updated.Connection.PostgresPassword != "editedpass" {
		t.Fatalf("unexpected updated postgres config: %+v", updated.Connection)
	}
	if updated.Connection.NATSToken != "editedtoken" {
		t.Fatalf("unexpected updated nats config: %+v", updated.Connection)
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
		"nats://editedtoken@localhost:24222",
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
