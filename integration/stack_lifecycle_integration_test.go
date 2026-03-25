//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	Status          string `json:"status"`
	ContainerName   string `json:"container_name"`
	Image           string `json:"image"`
	DataVolume      string `json:"data_volume"`
	Host            string `json:"host"`
	ExternalPort    int    `json:"external_port"`
	InternalPort    int    `json:"internal_port"`
	Database        string `json:"database"`
	MaintenanceDB   string `json:"maintenance_database"`
	Email           string `json:"email"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	AppendOnly      *bool  `json:"appendonly"`
	SavePolicy      string `json:"save_policy"`
	MaxMemoryPolicy string `json:"maxmemory_policy"`
	ServerMode      string `json:"server_mode"`
	URL             string `json:"url"`
	DSN             string `json:"dsn"`
}

func TestManagedStackLifecycleWithCustomConfig(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("integration tests require Linux")
	}

	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot, err := os.MkdirTemp("", "stackctl-itest-data-*")
	if err != nil {
		t.Fatalf("create integration data dir: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	requirePodmanCompose(t)

	env := []string{
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
	cfg.Services.NATSContainer = "stackctl-it-nats-" + suffix
	cfg.Services.PgAdminContainer = "stackctl-it-pgadmin-" + suffix
	cfg.Services.Postgres.DataVolume = "stackctl_it_postgres_data_" + suffix
	cfg.Services.Redis.DataVolume = "stackctl_it_redis_data_" + suffix
	cfg.Services.Redis.AppendOnly = true
	cfg.Services.Redis.SavePolicy = "900 1 300 10"
	cfg.Services.Redis.MaxMemoryPolicy = "allkeys-lru"
	cfg.Services.PgAdmin.DataVolume = "stackctl_it_pgadmin_data_" + suffix
	cfg.Services.PgAdmin.ServerMode = true
	cfg.Connection.Host = "127.0.0.1"
	cfg.Connection.PostgresDatabase = "stackdb"
	cfg.Connection.PostgresUsername = "stackuser"
	cfg.Connection.PostgresPassword = "stackpass"
	cfg.Connection.RedisPassword = "redispass"
	cfg.Connection.NATSToken = "natspass"
	cfg.Connection.PgAdminEmail = "pgadmin@example.com"
	cfg.Connection.PgAdminPassword = "pgsecret"
	cfg.Ports.Postgres = freePort(t)
	cfg.Ports.Redis = freePort(t)
	cfg.Ports.NATS = freePort(t)
	cfg.Ports.PgAdmin = freePort(t)
	cfg.Ports.Cockpit = freePort(t)
	cfg.Behavior.StartupTimeoutSec = 240
	cfg.ApplyDerivedFields()

	if _, err := configpkg.ScaffoldManagedStack(cfg, true); err != nil {
		t.Fatalf("scaffold managed compose file: %v", err)
	}

	cfg.Stack.Managed = false
	cfg.Setup.ScaffoldDefaultStack = false
	if err := configpkg.Save("", cfg); err != nil {
		t.Fatalf("save integration config: %v", err)
	}

	t.Cleanup(func() {
		_, _ = runStackctl(binaryPath, env, "reset", "--volumes", "--force")
		_, _ = runCommand("podman", "unshare", "rm", "-rf", dataRoot)
		_ = os.RemoveAll(dataRoot)
	})

	output, err := runStackctl(binaryPath, env, "config", "validate")
	if err != nil {
		t.Fatalf("config validate returned error: %v\n%s", err, output)
	}
	if !strings.Contains(output, "config is valid") {
		t.Fatalf("unexpected config validate output:\n%s", output)
	}

	startOutput, err := runStackctl(binaryPath, env, "start")
	if err != nil {
		statusOutput, _ := runStackctl(binaryPath, env, "status", "--json")
		servicesOutput, _ := runStackctl(binaryPath, env, "services")
		logsOutput, _ := runStackctl(binaryPath, env, "logs", "-s", "postgres", "-n", "50")
		t.Fatalf(
			"start returned error: %v\n%s\nstatus --json:\n%s\nservices:\n%s\npostgres logs:\n%s",
			err,
			startOutput,
			statusOutput,
			servicesOutput,
			logsOutput,
		)
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
	if len(servicesByName) != 5 {
		t.Fatalf("expected 5 services, got %d", len(servicesByName))
	}
	if postgres := servicesByName["postgres"]; postgres.Status != "running" || postgres.DSN != "postgres://stackuser:stackpass@127.0.0.1:"+strconv.Itoa(cfg.Ports.Postgres)+"/stackdb" {
		t.Fatalf("unexpected postgres service: %+v", postgres)
	}
	if postgres := servicesByName["postgres"]; postgres.DataVolume != cfg.Services.Postgres.DataVolume || postgres.MaintenanceDB != cfg.Services.Postgres.MaintenanceDatabase {
		t.Fatalf("unexpected postgres config: %+v", postgres)
	}
	if redis := servicesByName["redis"]; redis.Status != "running" || redis.DSN != "redis://:redispass@127.0.0.1:"+strconv.Itoa(cfg.Ports.Redis) {
		t.Fatalf("unexpected redis service: %+v", redis)
	}
	if redis := servicesByName["redis"]; redis.DataVolume != cfg.Services.Redis.DataVolume || redis.AppendOnly == nil || !*redis.AppendOnly || redis.SavePolicy != cfg.Services.Redis.SavePolicy || redis.MaxMemoryPolicy != cfg.Services.Redis.MaxMemoryPolicy {
		t.Fatalf("unexpected redis config: %+v", redis)
	}
	if nats := servicesByName["nats"]; nats.Status != "running" || nats.DSN != "nats://natspass@127.0.0.1:"+strconv.Itoa(cfg.Ports.NATS) {
		t.Fatalf("unexpected nats service: %+v", nats)
	}
	if pgadmin := servicesByName["pgadmin"]; pgadmin.Status != "running" || pgadmin.Email != "pgadmin@example.com" || pgadmin.Password != "" {
		t.Fatalf("unexpected pgadmin service: %+v", pgadmin)
	}
	if pgadmin := servicesByName["pgadmin"]; pgadmin.DataVolume != cfg.Services.PgAdmin.DataVolume || pgadmin.ServerMode != "enabled" {
		t.Fatalf("unexpected pgadmin config: %+v", pgadmin)
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
		"nats://natspass@127.0.0.1:" + strconv.Itoa(cfg.Ports.NATS),
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
	if len(containers) != 4 {
		t.Fatalf("expected 4 stack containers, got %d", len(containers))
	}

	healthOutput, err := runStackctl(binaryPath, env, "health")
	if err != nil {
		t.Fatalf("health returned error: %v\n%s", err, healthOutput)
	}
	for _, fragment := range []string{
		"postgres port listening",
		"redis port listening",
		"nats port listening",
		"pgadmin port listening",
		"postgres running",
		"redis running",
		"nats running",
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
		"NATS",
		"pgAdmin",
		"Cockpit",
		strconv.Itoa(cfg.Ports.Postgres) + " -> 5432",
		strconv.Itoa(cfg.Ports.Redis) + " -> 6379",
		strconv.Itoa(cfg.Ports.NATS) + " -> 4222",
		strconv.Itoa(cfg.Ports.PgAdmin) + " -> 80",
		strconv.Itoa(cfg.Ports.Cockpit) + " -> 9090",
	} {
		if !strings.Contains(portsOutput, fragment) {
			t.Fatalf("ports output missing %q:\n%s", fragment, portsOutput)
		}
	}

	invalidLogsOutput, err := runStackctl(binaryPath, env, "logs", "-s", "invalid")
	if err == nil || !strings.Contains(invalidLogsOutput, "valid values: postgres, redis, nats, pgadmin") {
		t.Fatalf("expected invalid service error, got err=%v output=%s", err, invalidLogsOutput)
	}

	natsLogsOutput, err := runStackctl(binaryPath, env, "logs", "-s", "nats", "-n", "5")
	if err != nil {
		t.Fatalf("logs -s nats returned error: %v\n%s", err, natsLogsOutput)
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
			"redis",
			"--",
			"redis-cli",
			"-a", cfg.Connection.RedisPassword,
			"CONFIG",
			"GET",
			"appendonly",
			"save",
			"maxmemory-policy",
		)
		if err != nil {
			return err
		}
		for _, fragment := range []string{"appendonly", "yes", "save", "900 1 300 10", "maxmemory-policy", "allkeys-lru"} {
			if !strings.Contains(output, fragment) {
				return fmt.Errorf("unexpected redis config output: %q", output)
			}
		}
		return nil
	})

	dumpPath := dataRoot + "/stackctl-test-dump.sql"

	setupDumpOutput, err := runStackctl(
		binaryPath,
		env,
		"db",
		"shell",
		"--",
		"-v", "ON_ERROR_STOP=1",
		"-c", "CREATE TABLE IF NOT EXISTS stackctl_restore_test (id integer primary key, value text not null); TRUNCATE stackctl_restore_test; INSERT INTO stackctl_restore_test(id, value) VALUES (1, 'restored')",
	)
	if err != nil {
		t.Fatalf("prepare db dump state: %v\n%s", err, setupDumpOutput)
	}

	dumpOutput, err := runStackctl(binaryPath, env, "db", "dump", dumpPath)
	if err != nil {
		t.Fatalf("db dump returned error: %v\n%s", err, dumpOutput)
	}
	if !strings.Contains(dumpOutput, "wrote database dump to "+dumpPath) {
		t.Fatalf("unexpected db dump output:\n%s", dumpOutput)
	}

	dumpData, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatalf("read database dump: %v", err)
	}
	if !strings.Contains(string(dumpData), "stackctl_restore_test") {
		t.Fatalf("database dump missing test table:\n%s", string(dumpData))
	}

	resetDBOutput, err := runStackctl(binaryPath, env, "db", "reset", "--force")
	if err != nil {
		t.Fatalf("db reset returned error: %v\n%s", err, resetDBOutput)
	}
	if !strings.Contains(resetDBOutput, "database stackdb reset") {
		t.Fatalf("unexpected db reset output:\n%s", resetDBOutput)
	}

	verifyResetOutput, err := runStackctl(
		binaryPath,
		env,
		"db",
		"shell",
		"--",
		"-tAc",
		"select coalesce(to_regclass('public.stackctl_restore_test')::text, '')",
	)
	if err != nil {
		t.Fatalf("verify db reset: %v\n%s", err, verifyResetOutput)
	}
	if strings.TrimSpace(verifyResetOutput) != "" {
		t.Fatalf("expected reset database to remove the test table, got %q", verifyResetOutput)
	}

	restoreOutput, err := runStackctl(binaryPath, env, "db", "restore", dumpPath, "--force")
	if err != nil {
		t.Fatalf("db restore returned error: %v\n%s", err, restoreOutput)
	}
	if !strings.Contains(restoreOutput, "database restore completed") {
		t.Fatalf("unexpected db restore output:\n%s", restoreOutput)
	}

	verifyRestoreOutput, err := runStackctl(
		binaryPath,
		env,
		"db",
		"shell",
		"--",
		"-tAc",
		"select value from stackctl_restore_test where id = 1",
	)
	if err != nil {
		t.Fatalf("verify db restore: %v\n%s", err, verifyRestoreOutput)
	}
	if strings.TrimSpace(verifyRestoreOutput) != "restored" {
		t.Fatalf("unexpected restored row: %q", verifyRestoreOutput)
	}

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

	serverModeOutput, err := runStackctl(
		binaryPath,
		env,
		"exec",
		"pgadmin",
		"--",
		"printenv",
		"PGADMIN_CONFIG_SERVER_MODE",
	)
	if err != nil {
		t.Fatalf("read pgadmin server mode: %v\n%s", err, serverModeOutput)
	}
	if strings.TrimSpace(serverModeOutput) != "True" {
		t.Fatalf("unexpected pgadmin server mode: %q", serverModeOutput)
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

func TestNamedStackSelectionAndPathResolution(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("integration tests require Linux")
	}

	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot, err := os.MkdirTemp("", "stackctl-itest-data-*")
	if err != nil {
		t.Fatalf("create integration data dir: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	requirePodmanCompose(t)

	env := []string{
		"XDG_CONFIG_HOME=" + configRoot,
		"XDG_DATA_HOME=" + dataRoot,
	}

	suffix := strings.ToLower(strconv.FormatInt(time.Now().UnixNano(), 36))
	alphaName := "alpha-" + suffix
	betaName := "beta-" + suffix

	alphaCfg := integrationNATSOnlyStackConfig(t, alphaName, "alpha-token-"+suffix)
	betaCfg := integrationNATSOnlyStackConfig(t, betaName, "beta-token-"+suffix)

	alphaPath, err := configpkg.ConfigFilePathForStack(alphaName)
	if err != nil {
		t.Fatalf("resolve alpha config path: %v", err)
	}
	betaPath, err := configpkg.ConfigFilePathForStack(betaName)
	if err != nil {
		t.Fatalf("resolve beta config path: %v", err)
	}

	if err := configpkg.Save(alphaPath, alphaCfg); err != nil {
		t.Fatalf("save alpha config: %v", err)
	}
	if err := configpkg.Save(betaPath, betaCfg); err != nil {
		t.Fatalf("save beta config: %v", err)
	}

	t.Cleanup(func() {
		_, _ = runStackctl(binaryPath, env, "--stack", alphaName, "reset", "--volumes", "--force")
		_, _ = runStackctl(binaryPath, env, "--stack", betaName, "reset", "--volumes", "--force")
		_, _ = runCommand("podman", "unshare", "rm", "-rf", dataRoot)
		_ = os.RemoveAll(dataRoot)
	})

	listOutput, err := runStackctl(binaryPath, env, "stack", "list")
	if err != nil {
		t.Fatalf("stack list returned error: %v\n%s", err, listOutput)
	}
	for _, fragment := range []string{alphaName, betaName} {
		if !strings.Contains(listOutput, fragment) {
			t.Fatalf("stack list missing %q:\n%s", fragment, listOutput)
		}
	}

	useOutput, err := runStackctl(binaryPath, env, "stack", "use", alphaName)
	if err != nil {
		t.Fatalf("stack use returned error: %v\n%s", err, useOutput)
	}
	if !strings.Contains(useOutput, "selected stack "+alphaName) {
		t.Fatalf("unexpected stack use output:\n%s", useOutput)
	}

	currentOutput, err := runStackctl(binaryPath, env, "stack", "current")
	if err != nil {
		t.Fatalf("stack current returned error: %v\n%s", err, currentOutput)
	}
	if strings.TrimSpace(currentOutput) != alphaName {
		t.Fatalf("expected current stack %q, got %q", alphaName, strings.TrimSpace(currentOutput))
	}

	pathOutput, err := runStackctl(binaryPath, env, "config", "path")
	if err != nil {
		t.Fatalf("config path returned error: %v\n%s", err, pathOutput)
	}
	if strings.TrimSpace(pathOutput) != alphaPath {
		t.Fatalf("expected alpha config path %q, got %q", alphaPath, strings.TrimSpace(pathOutput))
	}

	overridePathOutput, err := runStackctl(binaryPath, env, "--stack", betaName, "config", "path")
	if err != nil {
		t.Fatalf("--stack config path returned error: %v\n%s", err, overridePathOutput)
	}
	if strings.TrimSpace(overridePathOutput) != betaPath {
		t.Fatalf("expected beta config path %q, got %q", betaPath, strings.TrimSpace(overridePathOutput))
	}

	currentAfterOverride, err := runStackctl(binaryPath, env, "stack", "current")
	if err != nil {
		t.Fatalf("stack current after override returned error: %v\n%s", err, currentAfterOverride)
	}
	if strings.TrimSpace(currentAfterOverride) != alphaName {
		t.Fatalf("expected saved selection to remain %q, got %q", alphaName, strings.TrimSpace(currentAfterOverride))
	}
}

func TestNamedStackSingleRunningStackGuard(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("integration tests require Linux")
	}

	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot, err := os.MkdirTemp("", "stackctl-itest-data-*")
	if err != nil {
		t.Fatalf("create integration data dir: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	requirePodmanCompose(t)

	env := []string{
		"XDG_CONFIG_HOME=" + configRoot,
		"XDG_DATA_HOME=" + dataRoot,
	}

	suffix := strings.ToLower(strconv.FormatInt(time.Now().UnixNano(), 36))
	alphaName := "alpha-" + suffix
	betaName := "beta-" + suffix

	alphaCfg := integrationNATSOnlyStackConfig(t, alphaName, "alpha-token-"+suffix)
	betaCfg := integrationNATSOnlyStackConfig(t, betaName, "beta-token-"+suffix)

	alphaPath, err := configpkg.ConfigFilePathForStack(alphaName)
	if err != nil {
		t.Fatalf("resolve alpha config path: %v", err)
	}
	betaPath, err := configpkg.ConfigFilePathForStack(betaName)
	if err != nil {
		t.Fatalf("resolve beta config path: %v", err)
	}

	if err := configpkg.Save(alphaPath, alphaCfg); err != nil {
		t.Fatalf("save alpha config: %v", err)
	}
	if err := configpkg.Save(betaPath, betaCfg); err != nil {
		t.Fatalf("save beta config: %v", err)
	}

	t.Cleanup(func() {
		_, _ = runStackctl(binaryPath, env, "--stack", alphaName, "reset", "--volumes", "--force")
		_, _ = runStackctl(binaryPath, env, "--stack", betaName, "reset", "--volumes", "--force")
		_, _ = runCommand("podman", "unshare", "rm", "-rf", dataRoot)
		_ = os.RemoveAll(dataRoot)
	})

	startAlphaOutput, err := runStackctl(binaryPath, env, "--stack", alphaName, "start")
	if err != nil {
		t.Fatalf("alpha start returned error: %v\n%s", err, startAlphaOutput)
	}
	alphaDSN := "nats://alpha-token-" + suffix + "@127.0.0.1:" + strconv.Itoa(alphaCfg.Ports.NATS)
	if !strings.Contains(startAlphaOutput, alphaDSN) {
		t.Fatalf("alpha start output missing DSN %q:\n%s", alphaDSN, startAlphaOutput)
	}

	startBetaWhileAlphaRunning, err := runStackctl(binaryPath, env, "--stack", betaName, "start")
	if err == nil {
		t.Fatalf("expected beta start to fail while alpha is running:\n%s", startBetaWhileAlphaRunning)
	}
	if !strings.Contains(startBetaWhileAlphaRunning, "another local stack is already running: "+alphaName) {
		t.Fatalf("expected running-stack guard in output:\n%s", startBetaWhileAlphaRunning)
	}
	if !strings.Contains(startBetaWhileAlphaRunning, "`stackctl --stack "+alphaName+" stop`") {
		t.Fatalf("expected actionable stop guidance in output:\n%s", startBetaWhileAlphaRunning)
	}

	stopAlphaOutput, err := runStackctl(binaryPath, env, "--stack", alphaName, "stop")
	if err != nil {
		t.Fatalf("alpha stop returned error: %v\n%s", err, stopAlphaOutput)
	}
	if !strings.Contains(stopAlphaOutput, "stack stopped") {
		t.Fatalf("unexpected alpha stop output:\n%s", stopAlphaOutput)
	}

	startBetaOutput, err := runStackctl(binaryPath, env, "--stack", betaName, "start")
	if err != nil {
		t.Fatalf("beta start returned error: %v\n%s", err, startBetaOutput)
	}
	betaDSN := "nats://beta-token-" + suffix + "@127.0.0.1:" + strconv.Itoa(betaCfg.Ports.NATS)
	if !strings.Contains(startBetaOutput, betaDSN) {
		t.Fatalf("beta start output missing DSN %q:\n%s", betaDSN, startBetaOutput)
	}
}

func TestNamedStackCloneRenameUseAndDeleteLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("integration tests require Linux")
	}

	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot, err := os.MkdirTemp("", "stackctl-itest-data-*")
	if err != nil {
		t.Fatalf("create integration data dir: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	requirePodmanCompose(t)

	env := []string{
		"XDG_CONFIG_HOME=" + configRoot,
		"XDG_DATA_HOME=" + dataRoot,
	}

	suffix := strings.ToLower(strconv.FormatInt(time.Now().UnixNano(), 36))
	sourceName := "source-" + suffix
	clonedName := "cloned-" + suffix
	renamedName := "renamed-" + suffix

	sourceCfg := integrationNATSOnlyStackConfig(t, sourceName, "clone-token-"+suffix)

	sourcePath, err := configpkg.ConfigFilePathForStack(sourceName)
	if err != nil {
		t.Fatalf("resolve source config path: %v", err)
	}
	clonedPath, err := configpkg.ConfigFilePathForStack(clonedName)
	if err != nil {
		t.Fatalf("resolve cloned config path: %v", err)
	}
	renamedPath, err := configpkg.ConfigFilePathForStack(renamedName)
	if err != nil {
		t.Fatalf("resolve renamed config path: %v", err)
	}
	clonedDir, err := configpkg.ManagedStackDir(clonedName)
	if err != nil {
		t.Fatalf("resolve cloned stack dir: %v", err)
	}
	renamedDir, err := configpkg.ManagedStackDir(renamedName)
	if err != nil {
		t.Fatalf("resolve renamed stack dir: %v", err)
	}

	if err := configpkg.Save(sourcePath, sourceCfg); err != nil {
		t.Fatalf("save source config: %v", err)
	}

	t.Cleanup(func() {
		_, _ = runStackctl(binaryPath, env, "--stack", sourceName, "reset", "--volumes", "--force")
		_, _ = runStackctl(binaryPath, env, "--stack", clonedName, "reset", "--volumes", "--force")
		_, _ = runStackctl(binaryPath, env, "--stack", renamedName, "reset", "--volumes", "--force")
		_, _ = runCommand("podman", "unshare", "rm", "-rf", dataRoot)
		_ = os.RemoveAll(dataRoot)
	})

	cloneOutput, err := runStackctl(binaryPath, env, "stack", "clone", sourceName, clonedName)
	if err != nil {
		t.Fatalf("stack clone returned error: %v\n%s", err, cloneOutput)
	}
	if !strings.Contains(cloneOutput, "cloned stack "+sourceName+" to "+clonedName) {
		t.Fatalf("unexpected stack clone output:\n%s", cloneOutput)
	}
	if !strings.Contains(cloneOutput, "new config written to "+clonedPath) {
		t.Fatalf("stack clone output missing config path:\n%s", cloneOutput)
	}

	clonedCfg, err := configpkg.Load(clonedPath)
	if err != nil {
		t.Fatalf("load cloned config: %v", err)
	}
	if clonedCfg.Stack.Name != clonedName {
		t.Fatalf("expected cloned stack name %q, got %q", clonedName, clonedCfg.Stack.Name)
	}
	if clonedCfg.Connection.NATSToken != sourceCfg.Connection.NATSToken {
		t.Fatalf("expected cloned NATS token %q, got %q", sourceCfg.Connection.NATSToken, clonedCfg.Connection.NATSToken)
	}
	if clonedCfg.Stack.Dir != clonedDir {
		t.Fatalf("expected cloned stack dir %q, got %q", clonedDir, clonedCfg.Stack.Dir)
	}
	if _, err := os.Stat(configpkg.ComposePath(clonedCfg)); err != nil {
		t.Fatalf("stat cloned compose file: %v", err)
	}

	renameOutput, err := runStackctl(binaryPath, env, "stack", "rename", clonedName, renamedName)
	if err != nil {
		t.Fatalf("stack rename returned error: %v\n%s", err, renameOutput)
	}
	if !strings.Contains(renameOutput, "renamed stack "+clonedName+" to "+renamedName) {
		t.Fatalf("unexpected stack rename output:\n%s", renameOutput)
	}
	if !strings.Contains(renameOutput, "wrote managed compose file "+filepath.Join(renamedDir, configpkg.DefaultComposeFileName)) {
		t.Fatalf("stack rename output missing scaffold refresh:\n%s", renameOutput)
	}

	if _, err := os.Stat(clonedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected cloned config path %s to be removed, got err=%v", clonedPath, err)
	}
	if _, err := os.Stat(clonedDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected cloned managed dir %s to be moved, got err=%v", clonedDir, err)
	}

	renamedCfg, err := configpkg.Load(renamedPath)
	if err != nil {
		t.Fatalf("load renamed config: %v", err)
	}
	if renamedCfg.Stack.Name != renamedName {
		t.Fatalf("expected renamed stack name %q, got %q", renamedName, renamedCfg.Stack.Name)
	}
	if renamedCfg.Connection.NATSToken != sourceCfg.Connection.NATSToken {
		t.Fatalf("expected renamed NATS token %q, got %q", sourceCfg.Connection.NATSToken, renamedCfg.Connection.NATSToken)
	}
	if renamedCfg.Stack.Dir != renamedDir {
		t.Fatalf("expected renamed stack dir %q, got %q", renamedDir, renamedCfg.Stack.Dir)
	}
	if _, err := os.Stat(configpkg.ComposePath(renamedCfg)); err != nil {
		t.Fatalf("stat renamed compose file: %v", err)
	}

	useOutput, err := runStackctl(binaryPath, env, "stack", "use", renamedName)
	if err != nil {
		t.Fatalf("stack use returned error: %v\n%s", err, useOutput)
	}
	if !strings.Contains(useOutput, "selected stack "+renamedName) {
		t.Fatalf("unexpected stack use output:\n%s", useOutput)
	}

	currentOutput, err := runStackctl(binaryPath, env, "stack", "current")
	if err != nil {
		t.Fatalf("stack current returned error: %v\n%s", err, currentOutput)
	}
	if strings.TrimSpace(currentOutput) != renamedName {
		t.Fatalf("expected current stack %q, got %q", renamedName, strings.TrimSpace(currentOutput))
	}

	deleteOutput, err := runStackctl(binaryPath, env, "stack", "delete", renamedName, "--purge-data", "--force")
	if err != nil {
		t.Fatalf("stack delete returned error: %v\n%s", err, deleteOutput)
	}
	for _, fragment := range []string{
		"deleted managed stack data " + renamedDir,
		"deleted stack config " + renamedPath,
		"selected stack reset to " + configpkg.DefaultStackName,
	} {
		if !strings.Contains(deleteOutput, fragment) {
			t.Fatalf("stack delete output missing %q:\n%s", fragment, deleteOutput)
		}
	}

	if _, err := os.Stat(renamedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected renamed config path %s to be removed, got err=%v", renamedPath, err)
	}
	if _, err := os.Stat(renamedDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected renamed managed dir %s to be removed, got err=%v", renamedDir, err)
	}

	currentAfterDelete, err := runStackctl(binaryPath, env, "stack", "current")
	if err != nil {
		t.Fatalf("stack current after delete returned error: %v\n%s", err, currentAfterDelete)
	}
	if strings.TrimSpace(currentAfterDelete) != configpkg.DefaultStackName {
		t.Fatalf("expected current stack to reset to %q, got %q", configpkg.DefaultStackName, strings.TrimSpace(currentAfterDelete))
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

func integrationNATSOnlyStackConfig(t *testing.T, stackName string, token string) configpkg.Config {
	t.Helper()

	cfg := configpkg.DefaultForStack(stackName)
	cfg.Connection.Host = "127.0.0.1"
	cfg.Connection.NATSToken = token
	cfg.Ports.NATS = freePort(t)
	cfg.Behavior.StartupTimeoutSec = 180
	cfg.Setup.IncludePostgres = false
	cfg.Setup.IncludeRedis = false
	cfg.Setup.IncludePgAdmin = false
	cfg.Setup.IncludeCockpit = false
	cfg.Setup.InstallCockpit = false
	cfg.Setup.IncludeNATS = true
	cfg.ApplyDerivedFields()

	if _, err := configpkg.ScaffoldManagedStack(cfg, true); err != nil {
		t.Fatalf("scaffold managed stack %s: %v", stackName, err)
	}

	return cfg
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
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
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
