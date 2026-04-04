package compose

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestLogsAndContainerLogsIncludeSinceAndFollowOptions(t *testing.T) {
	cfg, logPath := writeFakePodman(t)

	client := Client{Runner: system.Runner{Stdout: io.Discard, Stderr: io.Discard}}
	if err := client.Logs(context.Background(), cfg, 200, true, "5m", "postgres"); err != nil {
		t.Fatalf("Logs returned error: %v", err)
	}
	assertRecordedArgs(t, logPath, []string{
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		"logs",
		"-f",
		"--tail",
		"200",
		"--since",
		"5m",
		"postgres",
	})

	_, logPath = writeFakePodman(t)
	if err := client.ContainerLogs(context.Background(), "local-postgres", 25, true, "10m"); err != nil {
		t.Fatalf("ContainerLogs returned error: %v", err)
	}
	assertRecordedArgs(t, logPath, []string{
		"logs",
		"-f",
		"--tail",
		"25",
		"--since",
		"10m",
		"local-postgres",
	})
}

func TestListContainersUsesSystemCaptureWhenNoCaptureFuncIsProvided(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "compose" ]; then
  printf '>>>> Executing external compose provider "/usr/bin/docker-compose". Please refer to the documentation for details. <<<<\n'
  printf '[{"Id":"legacy-123","Image":"postgres:16","Name":" local-postgres ","Names":[""," local-postgres "],"Status":"Up","State":"running","Publishers":[{"PublishedPort":5432,"TargetPort":5432,"Protocol":"TCP"}],"CreatedAt":"now"}]'
fi
`
	cfg, _ := writeFakePodmanScriptInDir(t, dir, script)
	t.Setenv("PODMAN_COMPOSE_PROVIDER", "docker-compose")

	containers, err := ListContainers(context.Background(), cfg.Stack.Dir, configpkg.ComposePath(cfg), nil)
	if err != nil {
		t.Fatalf("ListContainers returned error: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("unexpected container count: %d", len(containers))
	}
	if containers[0].ID != "legacy-123" {
		t.Fatalf("unexpected legacy container id: %+v", containers[0])
	}
	if got := containers[0].Ports[0].Protocol; got != "tcp" {
		t.Fatalf("expected protocol to be normalized, got %q", got)
	}
}

func TestListContainersAndParseComposeOutputHandleErrorsAndEmptyOutput(t *testing.T) {
	cfg := configpkg.Default()
	t.Setenv("PODMAN_COMPOSE_PROVIDER", "docker-compose")

	_, err := ListContainers(context.Background(), cfg.Stack.Dir, configpkg.ComposePath(cfg), func(context.Context, string, string, ...string) (system.CommandResult, error) {
		return system.CommandResult{}, errors.New("capture failed")
	})
	if err == nil || !strings.Contains(err.Error(), "capture failed") {
		t.Fatalf("unexpected capture error: %v", err)
	}

	containers, err := parseComposeContainerOutput("   \n\n")
	if err != nil {
		t.Fatalf("parseComposeContainerOutput returned error: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected empty compose output to parse to no containers, got %+v", containers)
	}

	_, err = parseComposeContainerOutput("{not-json}")
	if err == nil || !strings.Contains(err.Error(), "parse compose status output") {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestComposeHelpersCoverProviderAndFilterBranches(t *testing.T) {
	t.Run("preferred provider uses env override", func(t *testing.T) {
		t.Setenv("PODMAN_COMPOSE_PROVIDER", "docker-compose")
		if got := PreferredProvider(); got != "docker-compose" {
			t.Fatalf("expected env override provider, got %q", got)
		}
	})

	t.Run("preferred provider empty when helper is absent", func(t *testing.T) {
		t.Setenv("PODMAN_COMPOSE_PROVIDER", "")
		t.Setenv("PATH", t.TempDir())
		if got := PreferredProvider(); got != "" {
			t.Fatalf("expected no preferred provider, got %q", got)
		}
	})

	t.Run("compose filter skips excluding lines", func(t *testing.T) {
		if !shouldSkipComposeLine("** excluding: demo") {
			t.Fatal("expected excluding line to be filtered")
		}
	})

	t.Run("compose filter handles nil targets", func(t *testing.T) {
		filter := newComposeNoiseFilter(nil)
		if n, err := filter.Write([]byte("ignored")); err != nil || n != len("ignored") {
			t.Fatalf("unexpected nil-target write result n=%d err=%v", n, err)
		}
		if err := filter.Flush(); err != nil {
			t.Fatalf("Flush returned error: %v", err)
		}
	})
}

func TestComposeContainerConversionCoversFallbackBranches(t *testing.T) {
	container := composeContainer{
		LegacyID:   "legacy-id",
		Image:      "redis:7",
		Name:       " fallback-name ",
		Names:      composeNames{"", " "},
		Status:     "Up",
		State:      "running",
		Ports:      composePorts{{HostPort: 6379, ContainerPort: 6379, Protocol: "tcp"}},
		Publishers: nil,
		CreatedAt:  "later",
	}

	mapped := container.toSystemContainer()
	if mapped.ID != "legacy-id" {
		t.Fatalf("expected legacy id fallback, got %+v", mapped)
	}
	if len(mapped.Names) != 1 || mapped.Names[0] != "fallback-name" {
		t.Fatalf("expected trimmed fallback name, got %+v", mapped.Names)
	}
	if len(mapped.Ports) != 1 || mapped.Ports[0].HostPort != 6379 {
		t.Fatalf("expected legacy ports fallback, got %+v", mapped.Ports)
	}
}

func TestFilteredRunnerFlushPropagatesWriterErrors(t *testing.T) {
	writer := failingWriter{err: errors.New("write failed")}
	runner, flush := filteredRunner(system.Runner{
		Stdout: writer,
		Stderr: io.Discard,
	})

	if _, err := runner.Stdout.Write([]byte("line without newline")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := flush(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("unexpected flush error: %v", err)
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}
