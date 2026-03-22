//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
	"github.com/traweezy/stackctl/internal/testutil"
)

type runtimeServiceJSON struct {
	Name          string `json:"name"`
	DisplayName   string `json:"display_name"`
	Status        string `json:"status"`
	ContainerName string `json:"container_name"`
	Host          string `json:"host"`
	ExternalPort  int    `json:"external_port"`
	InternalPort  int    `json:"internal_port"`
	Database      string `json:"database"`
	Email         string `json:"email"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	URL           string `json:"url"`
	DSN           string `json:"dsn"`
}

func TestManagedStackLifecycleWithCustomConfig(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("integration tests require Linux")
	}

	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("HOME", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	requirePodmanCompose(t)

	env := []string{
		"HOME=" + dataRoot,
		"XDG_CONFIG_HOME=" + configRoot,
		"XDG_DATA_HOME=" + dataRoot,
	}

	suffix := strings.ToLower(strconv.FormatInt(time.Now().UnixNano(), 36))
	cfg := configpkg.Default()
	cfg.Stack.Name = "itest-" + suffix
	stackDir, err := configpkg.ManagedStackDir(cfg.Stack.Name)
	if err != nil {
		t.Fatalf("resolve managed stack dir: %v", err)
	}
	cfg.Stack.Dir = stackDir
	cfg.Services.PostgresContainer = "stackctl-it-postgres-" + suffix
	cfg.Services.RedisContainer = "stackctl-it-redis-" + suffix
	cfg.Services.PgAdminContainer = "stackctl-it-pgadmin-" + suffix
	cfg.Connection.Host = "127.0.0.1"
	cfg.Connection.PostgresDatabase = "stackdb"
	cfg.Connection.PostgresUsername = "stackuser"
	cfg.Connection.PostgresPassword = "stackpass"
	cfg.Connection.RedisPassword = "redispass"
	cfg.Connection.PgAdminEmail = "pgadmin@example.com"
	cfg.Connection.PgAdminPassword = "pgsecret"
	cfg.Ports.Postgres = freePort(t)
	cfg.Ports.Redis = freePort(t)
	cfg.Ports.PgAdmin = freePort(t)
	cfg.Ports.Cockpit = freePort(t)
	cfg.Behavior.StartupTimeoutSec = 120
	cfg.ApplyDerivedFields()

	if err := configpkg.Save("", cfg); err != nil {
		t.Fatalf("save integration config: %v", err)
	}

	t.Cleanup(func() {
		_, _ = runStackctl(binaryPath, env, "reset", "--volumes", "--force")
	})

	output, err := runStackctl(binaryPath, env, "config", "scaffold", "--force")
	if err != nil {
		t.Fatalf("config scaffold returned error: %v\n%s", err, output)
	}
	if !strings.Contains(output, "wrote managed compose file") {
		t.Fatalf("unexpected config scaffold output:\n%s", output)
	}

	output, err = runStackctl(binaryPath, env, "config", "validate")
	if err != nil {
		t.Fatalf("config validate returned error: %v\n%s", err, output)
	}
	if !strings.Contains(output, "config is valid") {
		t.Fatalf("unexpected config validate output:\n%s", output)
	}

	startOutput, err := runStackctl(binaryPath, env, "start")
	if err != nil {
		t.Fatalf("start returned error: %v\n%s", err, startOutput)
	}
	if !strings.Contains(startOutput, "stack started") {
		t.Fatalf("unexpected start output:\n%s", startOutput)
	}
	if !strings.Contains(startOutput, "postgres://stackuser:stackpass@127.0.0.1:"+strconv.Itoa(cfg.Ports.Postgres)+"/stackdb") {
		t.Fatalf("start output missing postgres DSN:\n%s", startOutput)
	}

	servicesOutput, err := runStackctl(binaryPath, env, "services", "--json")
	if err != nil {
		t.Fatalf("services --json returned error: %v\n%s", err, servicesOutput)
	}

	var services []runtimeServiceJSON
	if err := json.Unmarshal([]byte(servicesOutput), &services); err != nil {
		t.Fatalf("parse services json: %v\n%s", err, servicesOutput)
	}
	servicesByName := make(map[string]runtimeServiceJSON, len(services))
	for _, service := range services {
		servicesByName[service.Name] = service
	}
	if len(servicesByName) != 4 {
		t.Fatalf("expected 4 services, got %d", len(servicesByName))
	}
	if postgres := servicesByName["postgres"]; postgres.Status != "running" || postgres.DSN != "postgres://stackuser:stackpass@127.0.0.1:"+strconv.Itoa(cfg.Ports.Postgres)+"/stackdb" {
		t.Fatalf("unexpected postgres service: %+v", postgres)
	}
	if redis := servicesByName["redis"]; redis.Status != "running" || redis.DSN != "redis://:redispass@127.0.0.1:"+strconv.Itoa(cfg.Ports.Redis) {
		t.Fatalf("unexpected redis service: %+v", redis)
	}
	if pgadmin := servicesByName["pgadmin"]; pgadmin.Status != "running" || pgadmin.Email != "pgadmin@example.com" || pgadmin.Password != "" {
		t.Fatalf("unexpected pgadmin service: %+v", pgadmin)
	}
	if cockpit := servicesByName["cockpit"]; cockpit.Status == "" || cockpit.URL != "https://127.0.0.1:"+strconv.Itoa(cfg.Ports.Cockpit) {
		t.Fatalf("unexpected cockpit service: %+v", cockpit)
	}

	connectOutput, err := runStackctl(binaryPath, env, "connect")
	if err != nil {
		t.Fatalf("connect returned error: %v\n%s", err, connectOutput)
	}
	for _, fragment := range []string{
		"postgres://stackuser:stackpass@127.0.0.1:" + strconv.Itoa(cfg.Ports.Postgres) + "/stackdb",
		"redis://:redispass@127.0.0.1:" + strconv.Itoa(cfg.Ports.Redis),
		"http://127.0.0.1:" + strconv.Itoa(cfg.Ports.PgAdmin),
		"https://127.0.0.1:" + strconv.Itoa(cfg.Ports.Cockpit),
	} {
		if !strings.Contains(connectOutput, fragment) {
			t.Fatalf("connect output missing %q:\n%s", fragment, connectOutput)
		}
	}

	statusOutput, err := runStackctl(binaryPath, env, "status", "--json")
	if err != nil {
		t.Fatalf("status --json returned error: %v\n%s", err, statusOutput)
	}
	var containers []system.Container
	if err := json.Unmarshal([]byte(statusOutput), &containers); err != nil {
		t.Fatalf("parse status json: %v\n%s", err, statusOutput)
	}
	if len(containers) != 3 {
		t.Fatalf("expected 3 stack containers, got %d", len(containers))
	}

	healthOutput, err := runStackctl(binaryPath, env, "health")
	if err != nil {
		t.Fatalf("health returned error: %v\n%s", err, healthOutput)
	}
	for _, fragment := range []string{
		"postgres port listening",
		"redis port listening",
		"pgadmin port listening",
		"postgres running",
		"redis running",
		"pgadmin running",
	} {
		if !strings.Contains(healthOutput, fragment) {
			t.Fatalf("health output missing %q:\n%s", fragment, healthOutput)
		}
	}

	logsOutput, err := runStackctl(binaryPath, env, "logs", "-n", "5")
	if err != nil {
		t.Fatalf("logs returned error: %v\n%s", err, logsOutput)
	}
	if strings.Contains(logsOutput, "Executing external compose provider") || strings.Contains(logsOutput, "Docker Compose version") {
		t.Fatalf("logs output should filter compose provider noise:\n%s", logsOutput)
	}

	serviceLogsOutput, err := runStackctl(binaryPath, env, "logs", "-s", "postgres", "-n", "5")
	if err != nil {
		t.Fatalf("logs -s postgres returned error: %v\n%s", err, serviceLogsOutput)
	}

	portsOutput, err := runStackctl(binaryPath, env, "ports")
	if err != nil {
		t.Fatalf("ports returned error: %v\n%s", err, portsOutput)
	}
	for _, fragment := range []string{
		"Postgres",
		"Redis",
		"pgAdmin",
		"Cockpit",
		strconv.Itoa(cfg.Ports.Postgres) + " -> 5432",
		strconv.Itoa(cfg.Ports.Redis) + " -> 6379",
		strconv.Itoa(cfg.Ports.PgAdmin) + " -> 80",
		strconv.Itoa(cfg.Ports.Cockpit) + " -> 9090",
	} {
		if !strings.Contains(portsOutput, fragment) {
			t.Fatalf("ports output missing %q:\n%s", fragment, portsOutput)
		}
	}

	invalidLogsOutput, err := runStackctl(binaryPath, env, "logs", "-s", "invalid")
	if err == nil || !strings.Contains(invalidLogsOutput, "valid values: postgres, redis, pgadmin") {
		t.Fatalf("expected invalid service error, got err=%v output=%s", err, invalidLogsOutput)
	}

	assertEventuallyCommand(t, 30*time.Second, func() error {
		output, err := runStackctl(
			binaryPath,
			env,
			"db",
			"shell",
			"--",
			"-tAc",
			"select current_user || ':' || current_database()",
		)
		if err != nil {
			return err
		}
		if strings.TrimSpace(output) != "stackuser:stackdb" {
			return fmt.Errorf("unexpected db shell identity: %q", output)
		}
		return nil
	})

	assertEventuallyCommand(t, 30*time.Second, func() error {
		output, err := runStackctl(
			binaryPath,
			env,
			"exec",
			"postgres",
			"--",
			"psql",
			"-h", "127.0.0.1",
			"-U", cfg.Connection.PostgresUsername,
			"-d", cfg.Connection.PostgresDatabase,
			"-tAc",
			"select current_user || ':' || current_database()",
		)
		if err != nil {
			return err
		}
		if strings.TrimSpace(output) != "stackuser:stackdb" {
			return fmt.Errorf("unexpected postgres identity: %q", output)
		}
		return nil
	})

	assertEventuallyCommand(t, 30*time.Second, func() error {
		output, err := runStackctl(
			binaryPath,
			env,
			"exec",
			"redis",
			"--",
			"redis-cli",
			"-a", cfg.Connection.RedisPassword,
			"PING",
		)
		if err != nil {
			return err
		}
		if !strings.Contains(output, "PONG") {
			return fmt.Errorf("unexpected redis ping output: %q", output)
		}
		return nil
	})

	emailOutput, err := runStackctl(
		binaryPath,
		env,
		"exec",
		"pgadmin",
		"--",
		"printenv",
		"PGADMIN_DEFAULT_EMAIL",
	)
	if err != nil {
		t.Fatalf("read pgadmin email: %v\n%s", err, emailOutput)
	}
	if strings.TrimSpace(emailOutput) != "pgadmin@example.com" {
		t.Fatalf("unexpected pgadmin email: %q", emailOutput)
	}

	passwordOutput, err := runStackctl(
		binaryPath,
		env,
		"exec",
		"pgadmin",
		"--",
		"printenv",
		"PGADMIN_DEFAULT_PASSWORD",
	)
	if err != nil {
		t.Fatalf("read pgadmin password: %v\n%s", err, passwordOutput)
	}
	if strings.TrimSpace(passwordOutput) != "pgsecret" {
		t.Fatalf("unexpected pgadmin password: %q", passwordOutput)
	}

	stopOutput, err := runStackctl(binaryPath, env, "stop")
	if err != nil {
		t.Fatalf("stop returned error: %v\n%s", err, stopOutput)
	}
	if !strings.Contains(stopOutput, "stack stopped") {
		t.Fatalf("unexpected stop output:\n%s", stopOutput)
	}

	statusAfterStop, err := runStackctl(binaryPath, env, "status", "--json")
	if err != nil {
		t.Fatalf("status after stop returned error: %v\n%s", err, statusAfterStop)
	}
	var stoppedContainers []system.Container
	if err := json.Unmarshal([]byte(statusAfterStop), &stoppedContainers); err != nil {
		t.Fatalf("parse stopped status json: %v\n%s", err, statusAfterStop)
	}
	if len(stoppedContainers) != 0 {
		t.Fatalf("expected no containers after stop, got %d", len(stoppedContainers))
	}
}

func requirePodmanCompose(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman is not installed")
	}
	if _, err := runCommand("podman", "compose", "version"); err != nil {
		t.Skipf("podman compose is not available: %v", err)
	}
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer func() { _ = listener.Close() }()

	return listener.Addr().(*net.TCPAddr).Port
}

func runStackctl(binaryPath string, env []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = testutil.RepoRoot()
	cmd.Env = testutil.MergeEnv(env)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("stackctl command timed out: %s", strings.Join(args, " "))
	}

	return string(output), err
}

func runCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out: %s %s", name, strings.Join(args, " "))
	}

	return string(output), err
}

func assertEventuallyCommand(t *testing.T, timeout time.Duration, fn func() error) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = fn()
		if lastErr == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
	if lastErr == nil {
		lastErr = errors.New("condition never became true")
	}
	t.Fatal(lastErr)
}
