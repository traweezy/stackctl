package cmd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestWaitForStackContainersRemovedBranches(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	t.Run("container lookup failure", func(t *testing.T) {
		want := errors.New("podman ps failed")

		withTestDeps(t, func(d *commandDeps) {
			d.anyContainerExists = func(context.Context, []string) (bool, error) {
				return false, want
			}
		})

		err := waitForStackContainersRemoved(context.Background(), cfg)
		if !errors.Is(err, want) {
			t.Fatalf("expected lookup error %v, got %v", want, err)
		}
	})

	t.Run("context cancellation while containers remain", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.anyContainerExists = func(context.Context, []string) (bool, error) {
				return true, nil
			}
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := waitForStackContainersRemoved(ctx, cfg)
		if err == nil || !strings.Contains(err.Error(), "stack containers were not removed completely") {
			t.Fatalf("unexpected wait error: %v", err)
		}
	})
}

func TestRestartPropagatesBlankLineWriteFailures(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
		}
		d.waitForPort = func(context.Context, int, time.Duration) error { return nil }
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 3})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"restart"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected restart blank-line write failure, got %v", err)
	}
}

func TestRestartPropagatesServiceConnectionWriteFailures(t *testing.T) {
	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()

	withTestDeps(t, func(d *commandDeps) {
		d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
			return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
		}
		d.waitForPort = func(context.Context, int, time.Duration) error { return nil }
	})

	root := NewRootCmd(NewApp())
	root.SetOut(&failingWriteBuffer{failAfter: 4})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"restart", "postgres"})

	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("expected restart connection write failure, got %v", err)
	}
}
