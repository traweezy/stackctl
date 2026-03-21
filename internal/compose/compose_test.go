package compose

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestComposeNoiseFilterSkipsProviderBanner(t *testing.T) {
	var out strings.Builder
	filter := newComposeNoiseFilter(&out)

	if _, err := filter.Write([]byte(">>>> Executing external compose provider \"/usr/bin/podman-compose\". Please see podman-compose(1) for how to disable this message. <<<<\nservice log line\n")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := filter.Flush(); err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}

	if got := out.String(); got != "service log line\n" {
		t.Fatalf("unexpected filtered output: %q", got)
	}
}

func TestComposeNoiseFilterSkipsANSIProviderBanner(t *testing.T) {
	var out strings.Builder
	filter := newComposeNoiseFilter(&out)

	input := "\x1b[4m>>>> Executing external compose provider \"/usr/libexec/docker/cli-plugins/docker-compose\". " +
		"Please refer to the documentation for details. <<<<\nservice log line\n\x1b[0m"
	if _, err := filter.Write([]byte(input)); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := filter.Flush(); err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}

	if got := out.String(); got != "service log line\n" {
		t.Fatalf("unexpected filtered output: %q", got)
	}
}

func TestComposeNoiseFilterFlushesPartialLine(t *testing.T) {
	var out strings.Builder
	filter := newComposeNoiseFilter(&out)

	if _, err := filter.Write([]byte("final partial line")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := filter.Flush(); err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}

	if got := out.String(); got != "final partial line" {
		t.Fatalf("unexpected flushed output: %q", got)
	}
}

func TestLogsRunsPodmanComposeLogsCommand(t *testing.T) {
	cfg, logPath := writeFakePodman(t)

	client := Client{Runner: system.Runner{Stdout: io.Discard, Stderr: io.Discard}}
	if err := client.Logs(context.Background(), cfg, 100, false, ""); err != nil {
		t.Fatalf("Logs returned error: %v", err)
	}

	assertRecordedArgs(t, logPath, []string{
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		"logs",
		"--tail",
		"100",
	})
}

func TestLogsRunsFollowComposeCommand(t *testing.T) {
	cfg, logPath := writeFakePodman(t)

	client := Client{Runner: system.Runner{Stdout: io.Discard, Stderr: io.Discard}}
	if err := client.Logs(context.Background(), cfg, 50, true, ""); err != nil {
		t.Fatalf("Logs returned error: %v", err)
	}

	assertRecordedArgs(t, logPath, []string{
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		"logs",
		"-f",
		"--tail",
		"50",
	})
}

func TestContainerLogsRunsPodmanLogsCommand(t *testing.T) {
	_, logPath := writeFakePodman(t)

	client := Client{Runner: system.Runner{Stdout: io.Discard, Stderr: io.Discard}}
	if err := client.ContainerLogs(context.Background(), "local-postgres", 50, false, ""); err != nil {
		t.Fatalf("ContainerLogs returned error: %v", err)
	}

	assertRecordedArgs(t, logPath, []string{
		"logs",
		"--tail",
		"50",
		"local-postgres",
	})
}

func TestDownRunsSingleComposeSubcommand(t *testing.T) {
	cfg, logPath := writeFakePodman(t)

	client := Client{Runner: system.Runner{Stdout: io.Discard, Stderr: io.Discard}}
	if err := client.Down(context.Background(), cfg, true); err != nil {
		t.Fatalf("Down returned error: %v", err)
	}

	assertRecordedArgs(t, logPath, []string{
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		"down",
		"-v",
	})
}

func writeFakePodman(t *testing.T) (configpkg.Config, string) {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "podman-args.log")
	scriptPath := filepath.Join(dir, "podman")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(logPath) + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}

	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	cfg := configpkg.Default()
	cfg.Stack.Dir = dir
	cfg.Stack.ComposeFile = "compose.yaml"
	if err := os.WriteFile(filepath.Join(dir, cfg.Stack.ComposeFile), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	return cfg, logPath
}

func assertRecordedArgs(t *testing.T, logPath string, want []string) {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read recorded args: %v", err)
	}

	got := strings.Fields(strings.TrimSpace(string(data)))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args:\n got: %q\nwant: %q", got, want)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
