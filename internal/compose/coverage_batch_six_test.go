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

func TestComposeCoverageBatchSix(t *testing.T) {
	t.Run("composeNames and composePorts cover remaining JSON edge cases", func(t *testing.T) {
		var names composeNames
		if err := names.UnmarshalJSON([]byte(`"`)); err == nil {
			t.Fatal("expected composeNames to reject an unterminated JSON string")
		}

		var ports composePorts
		if err := ports.UnmarshalJSON([]byte("null")); err != nil {
			t.Fatalf("composePorts null unmarshal returned error: %v", err)
		}
		if ports != nil {
			t.Fatalf("expected composePorts null payload to reset to nil, got %+v", ports)
		}
	})

	t.Run("DownPath includes volume removal when requested", func(t *testing.T) {
		cfg, logPath := writeFakePodman(t)

		client := Client{Runner: system.Runner{Stdout: io.Discard, Stderr: io.Discard}}
		if err := client.DownPath(context.Background(), cfg.Stack.Dir, "/tmp/custom-compose.yaml", true); err != nil {
			t.Fatalf("DownPath returned error: %v", err)
		}

		assertRecordedArgs(t, logPath, []string{
			"compose",
			"-f",
			"/tmp/custom-compose.yaml",
			"down",
			"-v",
		})
	})

	t.Run("ListContainers reports stdout and exit-code fallback details", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Stack.Dir = t.TempDir()
		cfg.Stack.ComposeFile = "compose.yaml"
		t.Setenv("PODMAN_COMPOSE_PROVIDER", "docker-compose")

		_, err := ListContainers(context.Background(), cfg.Stack.Dir, configpkg.ComposePath(cfg), func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{ExitCode: 125, Stdout: "compose failed on stdout"}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "compose failed on stdout") {
			t.Fatalf("expected stdout failure detail, got %v", err)
		}

		_, err = ListContainers(context.Background(), cfg.Stack.Dir, configpkg.ComposePath(cfg), func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{ExitCode: 42}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "exit code 42") {
			t.Fatalf("expected exit-code fallback detail, got %v", err)
		}
	})

	t.Run("parseComposeContainerOutput rejects invalid array payloads", func(t *testing.T) {
		if _, err := parseComposeContainerOutput(`[{"Name":"postgres",]`); err == nil || !strings.Contains(err.Error(), "parse compose status output") {
			t.Fatalf("expected parse error for invalid array output, got %v", err)
		}
	})

	t.Run("compose noise filter write propagates writer errors for ready lines", func(t *testing.T) {
		filter := newComposeNoiseFilter(failingWriter{err: errors.New("write failed")})
		if _, err := filter.Write([]byte("service log line\n")); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected writeReadyLines failure, got %v", err)
		}
	})
}
