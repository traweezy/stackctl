package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestHealthCmdPrintsHealthLines(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	stdout, _, err := executeRoot(t, "health")
	if err != nil {
		t.Fatalf("health command returned error: %v", err)
	}
	for _, fragment := range []string{
		"postgres port not listening",
		"postgres container not found",
		"pgadmin container not found",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("expected health output to contain %q:\n%s", fragment, stdout)
		}
	}
}

func TestHealthCmdPropagatesRuntimeConfigErrors(t *testing.T) {
	expectedErr := errors.New("load failed")
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, expectedErr }
	})

	_, _, err := executeRoot(t, "health")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected runtime config error %v, got %v", expectedErr, err)
	}
}

func TestHealthCmdWatchModeRepeatsUntilCancelled(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: "[]"}, nil
		}
	})

	originalNotifyContext := healthNotifyContext
	originalTicker := newHealthTicker
	t.Cleanup(func() {
		healthNotifyContext = originalNotifyContext
		newHealthTicker = originalTicker
	})

	ctx, cancel := context.WithCancel(context.Background())
	healthNotifyContext = func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
		return ctx, cancel
	}

	tickC := make(chan time.Time, 1)
	newHealthTicker = func(time.Duration) (<-chan time.Time, func()) {
		return tickC, func() {}
	}

	cmd := newHealthCmd()
	if err := cmd.Flags().Set("watch", "true"); err != nil {
		t.Fatalf("set watch flag: %v", err)
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	done := make(chan error, 1)
	go func() {
		done <- cmd.RunE(cmd, nil)
	}()

	tickC <- time.Now()
	time.Sleep(10 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("health watch command returned error: %v", err)
	}

	output := stdout.String()
	if strings.Count(output, "postgres container not found") < 2 {
		t.Fatalf("expected repeated watch output, got:\n%s", output)
	}
}
