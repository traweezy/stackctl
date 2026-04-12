package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestPrintServicesInfoRendersAdditionalRuntimeLabels(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Services.Postgres.LogMinDurationStatementMS = 250
	cfg.Services.Redis.AppendOnly = false
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres", "redis")}, nil
		}
		d.portListening = func(int) bool { return true }
		d.portInUse = func(int) (bool, error) { return false, nil }
	})

	cmd := &cobra.Command{Use: "services"}
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := printServicesInfo(cmd, cfg); err != nil {
		t.Fatalf("printServicesInfo returned error: %v", err)
	}

	text := out.String()
	for _, fragment := range []string{
		"Image: docker.io/library/postgres:16",
		"Data volume: postgres_data",
		"Host: localhost",
		"Log min duration: 250 ms",
		"Maintenance DB: postgres",
		"Appendonly: disabled",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected services output to contain %q:\n%s", fragment, text)
		}
	}
}

func TestPrintServicesInfoPropagatesRuntimeLoadErrors(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{}, errors.New("ps failed")
		}
	})

	cmd := &cobra.Command{Use: "services"}
	err := printServicesInfo(cmd, cfg)
	if err == nil || !strings.Contains(err.Error(), "ps failed") {
		t.Fatalf("unexpected runtime load error: %v", err)
	}
}
