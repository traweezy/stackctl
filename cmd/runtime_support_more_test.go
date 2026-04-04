package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/creack/pty/v2"
	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestPrintServicesInfoRendersServiceDetails(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	definitions := enabledServiceDefinitions(cfg)
	containers := make([]system.Container, 0, len(definitions))
	for _, definition := range definitions {
		if definition.ContainerName == nil || definition.PrimaryPort == nil {
			continue
		}
		containers = append(containers, system.Container{
			ID:     definition.Key + "-id",
			Image:  definition.Key + ":latest",
			Names:  []string{definition.ContainerName(cfg)},
			Status: "Up 5 minutes",
			State:  "running",
			Ports: []system.ContainerPort{{
				HostPort:      definition.PrimaryPort(cfg),
				ContainerPort: definition.DefaultInternalPort,
				Protocol:      "tcp",
			}},
		})
	}

	containerJSON, err := json.Marshal(containers)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: string(containerJSON)}, nil
		}
		d.portListening = func(int) bool { return true }
		d.portInUse = func(int) (bool, error) { return false, nil }
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: true, Active: true, State: "active"}
		}
	})

	cmd := &cobra.Command{Use: "services"}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := printServicesInfo(cmd, cfg); err != nil {
		t.Fatalf("printServicesInfo returned error: %v", err)
	}

	text := out.String()
	for _, fragment := range []string{
		"Postgres",
		"Status: running",
		"Container: local-postgres",
		"Port: 5432",
		"Database: app",
		"Redis",
		"Cockpit",
		"URL: https://localhost:9090",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected services output to contain %q:\n%s", fragment, text)
		}
	}
}

func TestDefaultTerminalInteractiveReturnsTrueForPTY(t *testing.T) {
	stdinMaster, stdinTTY, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open stdin returned error: %v", err)
	}
	defer func() { _ = stdinMaster.Close() }()
	defer func() { _ = stdinTTY.Close() }()

	stdoutMaster, stdoutTTY, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open stdout returned error: %v", err)
	}
	defer func() { _ = stdoutMaster.Close() }()
	defer func() { _ = stdoutTTY.Close() }()

	originalStdin := os.Stdin
	originalStdout := os.Stdout
	os.Stdin = stdinTTY
	os.Stdout = stdoutTTY
	defer func() {
		os.Stdin = originalStdin
		os.Stdout = originalStdout
	}()

	if !defaultTerminalInteractive() {
		t.Fatal("expected PTY stdin/stdout to be detected as interactive")
	}
}
