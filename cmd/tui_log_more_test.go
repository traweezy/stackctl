package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
	stacktui "github.com/traweezy/stackctl/internal/tui"
)

func TestBuildTUILogWatchCommandRejectsMissingService(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, err := buildTUILogWatchCommand(stacktui.LogWatchRequest{})
	if err == nil || !strings.Contains(err.Error(), "live logs require a selected service") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildTUILogWatchCommandRequiresEnabledService(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, err := buildTUILogWatchCommand(stacktui.LogWatchRequest{Service: "pgadmin"})
	if err == nil || !strings.Contains(err.Error(), "pgadmin is not enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildTUILogWatchCommandWrapsMissingConfigHint(t *testing.T) {
	withTestDeps(t, nil)

	_, err := buildTUILogWatchCommand(stacktui.LogWatchRequest{Service: "postgres"})
	if err == nil || !strings.Contains(err.Error(), "stackctl setup") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildTUILogWatchCommandRequiresComposeFile(t *testing.T) {
	withTestDeps(t, func(value *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		value.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		value.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	})

	_, err := buildTUILogWatchCommand(stacktui.LogWatchRequest{Service: "postgres"})
	if err == nil || !strings.Contains(err.Error(), "compose file /tmp/stackctl/compose.yaml is not available") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTUILogWatchCommandRunPropagatesComposeLogErrors(t *testing.T) {
	cmd := &tuiLogWatchCommand{
		cfg:     configpkg.Default(),
		service: "postgres",
		stdin:   strings.NewReader(""),
		stdout:  io.Discard,
		stderr:  io.Discard,
	}

	withTestDeps(t, func(value *commandDeps) {
		value.composeLogs = func(context.Context, system.Runner, configpkg.Config, int, bool, string, string) error {
			return errors.New("log stream failed")
		}
	})

	if err := cmd.Run(); err == nil || !strings.Contains(err.Error(), "log stream failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
