package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestEnvPrintsShellAssignmentsFromConfig(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.Connection.PostgresPassword = "p@ss'word"
		cfg.Connection.RedisPassword = ""
		cfg.Setup.IncludeSeaweedFS = true
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(_ context.Context, _ string, _ string, _ ...string) (system.CommandResult, error) {
			t.Fatal("env should not inspect podman runtime")
			return system.CommandResult{}, nil
		}
		d.cockpitStatus = func(context.Context) system.CockpitState {
			t.Fatal("env should not inspect cockpit runtime")
			return system.CockpitState{}
		}
	})

	stdout, _, err := executeRoot(t, "env", "postgres", "redis", "seaweedfs")
	if err != nil {
		t.Fatalf("env returned error: %v", err)
	}

	for _, fragment := range []string{
		"# stackctl",
		"STACKCTL_STACK='dev-stack'",
		"# Postgres",
		"DATABASE_URL='postgres://app:p%40ss%27word@devbox:5432/app'",
		"PGHOST='devbox'",
		"POSTGRES_PASSWORD='p@ss'\"'\"'word'",
		"# Redis",
		"REDIS_PASSWORD=''",
		"# SeaweedFS",
		"S3_ENDPOINT='http://devbox:8333'",
		"AWS_ACCESS_KEY_ID='stackctl'",
		"AWS_SECRET_ACCESS_KEY='stackctlsecret'",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("expected env output to contain %q:\n%s", fragment, stdout)
		}
	}
	if strings.Contains(stdout, "# NATS") || strings.Contains(stdout, "# Cockpit") {
		t.Fatalf("did not expect unselected env groups:\n%s", stdout)
	}
}

func TestEnvExportPrefixesAssignments(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	stdout, _, err := executeRoot(t, "env", "--export", "postgres")
	if err != nil {
		t.Fatalf("env --export returned error: %v", err)
	}
	if !strings.Contains(stdout, "export STACKCTL_STACK='dev-stack'") {
		t.Fatalf("expected export prefix in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "export DATABASE_URL='postgres://app:app@localhost:5432/app'") {
		t.Fatalf("expected exported database url in output:\n%s", stdout)
	}
}

func TestEnvJSONPrintsMachineReadableMap(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Connection.Host = "devbox"
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	stdout, _, err := executeRoot(t, "env", "--json", "postgres", "cockpit")
	if err != nil {
		t.Fatalf("env --json returned error: %v", err)
	}

	values := make(map[string]string)
	if err := json.Unmarshal([]byte(stdout), &values); err != nil {
		t.Fatalf("unmarshal env json: %v\n%s", err, stdout)
	}

	if got := values["STACKCTL_STACK"]; got != "dev-stack" {
		t.Fatalf("unexpected STACKCTL_STACK: %q", got)
	}
	if got := values["DATABASE_URL"]; got != "postgres://app:app@devbox:5432/app" {
		t.Fatalf("unexpected DATABASE_URL: %q", got)
	}
	if got := values["COCKPIT_URL"]; got != "https://devbox:9090" {
		t.Fatalf("unexpected COCKPIT_URL: %q", got)
	}
	if _, ok := values["REDIS_URL"]; ok {
		t.Fatalf("did not expect unselected redis vars in %v", values)
	}
}

func TestEnvRejectsDisabledOrInvalidTargets(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.Setup.IncludePgAdmin = false
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, _, err := executeRoot(t, "env", "pgadmin")
	if err == nil || !strings.Contains(err.Error(), "pgadmin is not enabled") {
		t.Fatalf("unexpected disabled-target error: %v", err)
	}

	_, _, err = executeRoot(t, "env", "unknown")
	if err == nil || !strings.Contains(err.Error(), "invalid env target") {
		t.Fatalf("unexpected invalid-target error: %v", err)
	}
}

func TestEnvRejectsConflictingOutputModes(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
	})

	_, _, err := executeRoot(t, "env", "--json", "--export")
	if err == nil || !strings.Contains(err.Error(), "--json and --export cannot be used together") {
		t.Fatalf("unexpected conflicting-mode error: %v", err)
	}
}
