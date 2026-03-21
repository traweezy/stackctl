package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestOpenPrintsURLWhenBrowserLaunchFails(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		d.openURL = func(context.Context, system.Runner, string) error { return errors.New("no opener") }
	})

	stdout, _, err := executeRoot(t, "open", "cockpit")
	if err != nil {
		t.Fatalf("open returned error: %v", err)
	}
	if !strings.Contains(stdout, "[WARN] could not open cockpit automatically; use this URL") {
		t.Fatalf("stdout missing fallback warning: %s", stdout)
	}
	if !strings.Contains(stdout, "https://localhost:9090") {
		t.Fatalf("stdout missing fallback URL: %s", stdout)
	}
}
