package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestContainerNameForLogsCoversAliasesAndErrors(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	cases := []struct {
		service string
		want    string
	}{
		{service: "postgres", want: cfg.Services.PostgresContainer},
		{service: "pg", want: cfg.Services.PostgresContainer},
		{service: "redis", want: cfg.Services.RedisContainer},
		{service: "rd", want: cfg.Services.RedisContainer},
		{service: "nats", want: cfg.Services.NATSContainer},
		{service: "seaweedfs", want: cfg.Services.SeaweedFSContainer},
		{service: "seaweed", want: cfg.Services.SeaweedFSContainer},
		{service: "meilisearch", want: cfg.Services.MeilisearchContainer},
		{service: "meili", want: cfg.Services.MeilisearchContainer},
		{service: "pgadmin", want: cfg.Services.PgAdminContainer},
	}

	for _, tc := range cases {
		if got, err := containerNameForLogs(cfg, tc.service); err != nil || got != tc.want {
			t.Fatalf("%s: expected %q, got %q (%v)", tc.service, tc.want, got, err)
		}
	}

	cfg.Services.RedisContainer = ""
	if _, err := containerNameForLogs(cfg, "redis"); err == nil || !strings.Contains(err.Error(), `service "redis" does not define a container name`) {
		t.Fatalf("expected missing redis container error, got %v", err)
	}

	fresh := configpkg.Default()
	fresh.ApplyDerivedFields()
	if _, err := containerNameForLogs(fresh, "unknown"); err == nil || !strings.Contains(err.Error(), `invalid service "unknown"`) {
		t.Fatalf("expected invalid service error, got %v", err)
	}
}

func TestConfirmAutomaticFixCoversYesPromptAndErrorPaths(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	ok, err := confirmAutomaticFix(cmd, true, "Apply fixes?")
	if err != nil {
		t.Fatalf("confirmAutomaticFix returned error for --yes: %v", err)
	}
	if !ok {
		t.Fatal("expected confirmAutomaticFix to approve fixes with --yes")
	}

	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(_ io.Reader, _ io.Writer, _ string, _ bool) (bool, error) {
			return false, nil
		}
	})

	ok, err = confirmAutomaticFix(cmd, false, "Apply fixes?")
	if err != nil {
		t.Fatalf("confirmAutomaticFix returned error for declined prompt: %v", err)
	}
	if ok {
		t.Fatal("expected confirmAutomaticFix to return false when the user declines")
	}

	withTestDeps(t, func(d *commandDeps) {
		d.isTerminal = func() bool { return true }
		d.promptYesNo = func(_ io.Reader, _ io.Writer, _ string, _ bool) (bool, error) {
			return false, errors.New("input unavailable")
		}
	})

	ok, err = confirmAutomaticFix(cmd, false, "Apply fixes?")
	if err == nil || !strings.Contains(err.Error(), "automatic fix confirmation required; rerun with --yes") {
		t.Fatalf("unexpected confirmAutomaticFix error: %v", err)
	}
	if ok {
		t.Fatal("expected confirmAutomaticFix to reject the fix on prompt errors")
	}
}

func TestPrintDoctorReportIncludesSummaryAndOptionalMarkdown(t *testing.T) {
	report := newReport(
		doctorpkg.Check{Status: output.StatusOK, Message: "podman installed"},
		doctorpkg.Check{Status: output.StatusWarn, Message: "compose provider needs attention"},
	)

	t.Run("non-terminal skips markdown remediation", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return false }
		})

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		if err := printDoctorReport(cmd, report); err != nil {
			t.Fatalf("printDoctorReport returned error: %v", err)
		}

		text := stdout.String()
		if !strings.Contains(text, "Summary: 1 ok, 1 warn, 0 miss, 0 fail") {
			t.Fatalf("expected doctor summary in output, got:\n%s", text)
		}
		if strings.Contains(text, "## Suggested actions") {
			t.Fatalf("did not expect markdown remediation block for non-terminal output:\n%s", text)
		}
	})

	t.Run("terminal includes remediation markdown", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.isTerminal = func() bool { return true }
		})

		terminalReport := newReport(
			doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
			doctorpkg.Check{Status: output.StatusWarn, Message: "compose provider needs attention"},
		)

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		if err := printDoctorReport(cmd, terminalReport); err != nil {
			t.Fatalf("printDoctorReport returned error: %v", err)
		}

		text := stdout.String()
		if !strings.Contains(text, "## Suggested actions") {
			t.Fatalf("expected markdown remediation block in terminal output, got:\n%s", text)
		}
		if !strings.Contains(text, "stackctl doctor --fix --yes") {
			t.Fatalf("expected remediation guidance in terminal output, got:\n%s", text)
		}
	})
}

func TestPodmanVolumeExistsCoversExitCodesAndErrors(t *testing.T) {
	t.Run("existing volume", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{ExitCode: 0}, nil
			}
		})

		exists, err := podmanVolumeExists(context.Background(), "postgres_data")
		if err != nil {
			t.Fatalf("podmanVolumeExists returned error: %v", err)
		}
		if !exists {
			t.Fatal("expected podmanVolumeExists to detect an existing volume")
		}
	})

	t.Run("missing volume", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{ExitCode: 1}, nil
			}
		})

		exists, err := podmanVolumeExists(context.Background(), "redis_data")
		if err != nil {
			t.Fatalf("podmanVolumeExists returned error: %v", err)
		}
		if exists {
			t.Fatal("expected podmanVolumeExists to report a missing volume")
		}
	})

	t.Run("command error", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{}, errors.New("boom")
			}
		})

		_, err := podmanVolumeExists(context.Background(), "pgadmin_data")
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("expected capture error to surface, got %v", err)
		}
	})

	t.Run("unexpected exit code uses stderr fallback", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{ExitCode: 125, Stderr: "permission denied"}, nil
			}
		})

		_, err := podmanVolumeExists(context.Background(), "pgadmin_data")
		if err == nil || !strings.Contains(err.Error(), "check volume pgadmin_data: permission denied") {
			t.Fatalf("unexpected podmanVolumeExists error: %v", err)
		}
	})
}

func TestDisplayContainerStatusAndServicePendingReason(t *testing.T) {
	statusPreferred := stackServiceRuntimeState{
		ContainerStatus: "Exited (1) 10 seconds ago",
		ContainerState:  "exited",
	}
	if got := displayContainerStatus(statusPreferred); got != "Exited (1) 10 seconds ago" {
		t.Fatalf("expected container status text to win, got %q", got)
	}

	stateFallback := stackServiceRuntimeState{ContainerState: "running"}
	if got := displayContainerStatus(stateFallback); got != "running" {
		t.Fatalf("expected container state fallback, got %q", got)
	}

	if got := displayContainerStatus(stackServiceRuntimeState{}); got != "unknown" {
		t.Fatalf("expected unknown fallback, got %q", got)
	}

	cases := []struct {
		name  string
		state stackServiceRuntimeState
		want  string
	}{
		{
			name:  "missing container",
			state: stackServiceRuntimeState{Port: 5432},
			want:  "container not found",
		},
		{
			name: "stopped container",
			state: stackServiceRuntimeState{
				Port:             5432,
				ContainerFound:   true,
				ContainerRunning: false,
				ContainerStatus:  "Exited (1) 10 seconds ago",
			},
			want: "container status Exited (1) 10 seconds ago",
		},
		{
			name: "running without port mapping",
			state: stackServiceRuntimeState{
				Port:             5432,
				ContainerFound:   true,
				ContainerRunning: true,
				PortBound:        false,
			},
			want: "container is running but host port 5432 is not mapped",
		},
		{
			name: "waiting for listener",
			state: stackServiceRuntimeState{
				Port:             5432,
				ContainerFound:   true,
				ContainerRunning: true,
				PortBound:        true,
			},
			want: "port 5432 is not listening yet",
		},
	}

	for _, tc := range cases {
		if got := servicePendingReason(tc.state); got != tc.want {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestServiceStartFailureAndHelpers(t *testing.T) {
	if !terminalContainerState(" Exited ") {
		t.Fatal("expected exited state to be terminal")
	}
	if terminalContainerState("running") {
		t.Fatal("did not expect running state to be terminal")
	}

	portCheckErr := stackServiceRuntimeState{
		Definition: serviceDefinition{Key: "postgres"},
		Port:       5432,
		PortState:  stackServicePortState{CheckErr: errors.New("dial tcp timeout")},
	}
	if err := serviceStartFailure(portCheckErr); err == nil || !strings.Contains(err.Error(), "port 5432 check failed") {
		t.Fatalf("expected port check failure, got %v", err)
	}

	conflict := stackServiceRuntimeState{
		Definition: serviceDefinition{Key: "redis"},
		Port:       6379,
		PortState:  stackServicePortState{Conflict: true},
	}
	if err := serviceStartFailure(conflict); err == nil || !strings.Contains(err.Error(), "redis could not bind host port 6379") {
		t.Fatalf("expected port conflict failure, got %v", err)
	}

	stopped := stackServiceRuntimeState{
		Definition:      serviceDefinition{Key: "pgadmin"},
		Port:            5050,
		ContainerFound:  true,
		ContainerState:  "exited",
		ContainerStatus: "Exited (1) 5 seconds ago",
	}
	if err := serviceStartFailure(stopped); err == nil || !strings.Contains(err.Error(), "pgadmin container failed to start") {
		t.Fatalf("expected terminal container failure, got %v", err)
	}

	healthy := stackServiceRuntimeState{
		Definition:      serviceDefinition{Key: "postgres"},
		Port:            5432,
		ContainerFound:  true,
		ContainerState:  "running",
		ContainerStatus: "Up 5 seconds",
	}
	if err := serviceStartFailure(healthy); err != nil {
		t.Fatalf("expected no start failure for healthy state, got %v", err)
	}

	if err := firstServiceStartFailure([]stackServiceRuntimeState{healthy, conflict}); err == nil || !strings.Contains(err.Error(), "redis could not bind host port 6379") {
		t.Fatalf("expected firstServiceStartFailure to return the first failure, got %v", err)
	}
	if err := firstServiceStartFailure([]stackServiceRuntimeState{healthy}); err != nil {
		t.Fatalf("expected no aggregated failure for healthy states, got %v", err)
	}
}
