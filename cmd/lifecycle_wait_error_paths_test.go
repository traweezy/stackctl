package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestWaitForStackServiceReturnsWaitForPortFailures(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	definition, ok := serviceDefinitionByKey("postgres")
	if !ok {
		t.Fatal("expected postgres definition")
	}

	withTestDeps(t, func(d *commandDeps) {
		d.podmanComposeAvail = func(context.Context) bool { return false }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{
				Stdout: marshalContainersJSON(system.Container{
					ID:     "postgres-id",
					Image:  "postgres:latest",
					Names:  []string{cfg.Services.PostgresContainer},
					Status: "Up 5 minutes",
					State:  "running",
					Ports: []system.ContainerPort{{
						HostPort:      cfg.Ports.Postgres,
						ContainerPort: 5432,
						Protocol:      "tcp",
					}},
				}),
			}, nil
		}
		d.waitForPort = func(context.Context, int, time.Duration) error {
			return errors.New("probe failed")
		}
	})

	err := waitForStackService(context.Background(), cfg, definition)
	if err == nil || !strings.Contains(err.Error(), "postgres port 5432 did not become ready: probe failed") {
		t.Fatalf("unexpected waitForStackService error: %v", err)
	}
}

func TestWaitForStackServiceReturnsPendingReasonOnContextCancel(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	definition, ok := serviceDefinitionByKey("postgres")
	if !ok {
		t.Fatal("expected postgres definition")
	}

	withTestDeps(t, func(d *commandDeps) {
		d.podmanComposeAvail = func(context.Context) bool { return false }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: marshalContainersJSON()}, nil
		}
		d.portInUse = func(int) (bool, error) { return false, nil }
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForStackService(ctx, cfg, definition)
	if err == nil {
		t.Fatal("expected waitForStackService to fail for a cancelled context")
	}
	for _, fragment := range []string{
		"postgres did not become ready",
		"container not found",
		"context canceled",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected pending wait error to contain %q: %v", fragment, err)
		}
	}
}

func TestWaitForStackServicePropagatesContainerQueryErrors(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	definition, ok := serviceDefinitionByKey("postgres")
	if !ok {
		t.Fatal("expected postgres definition")
	}

	withTestDeps(t, func(d *commandDeps) {
		d.podmanComposeAvail = func(context.Context) bool { return false }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{}, errors.New("podman ps failed")
		}
	})

	err := waitForStackService(context.Background(), cfg, definition)
	if err == nil || !strings.Contains(err.Error(), "podman ps failed") {
		t.Fatalf("expected container query failure, got %v", err)
	}
}
