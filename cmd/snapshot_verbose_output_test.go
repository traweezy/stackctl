package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestSnapshotArchiveHelpersPropagateVerboseWriteErrors(t *testing.T) {
	originalOutput := rootOutput
	rootOutput.Verbose = true
	t.Cleanup(func() { rootOutput = originalOutput })

	cfg := configpkg.Default()
	cfg.ApplyDerivedFields()
	specs := []persistentVolumeSpec{{
		ServiceKey:   "postgres",
		DisplayName:  "Postgres",
		VolumeName:   "postgres_data",
		ArchiveEntry: "volumes/postgres.tar",
	}}

	t.Run("save snapshot export notice", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
				if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
					return system.CommandResult{ExitCode: 0}, nil
				}
				t.Fatal("snapshot save should stop before exporting when verbose output fails")
				return system.CommandResult{}, nil
			}
		})

		cmd := &cobra.Command{Use: "snapshot"}
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})

		err := saveSnapshotArchive(cmd, cfg, specs, filepath.Join(t.TempDir(), "snapshot.tar"))
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected verbose export write failure, got %v", err)
		}
	})

	t.Run("restore snapshot remove notice", func(t *testing.T) {
		payloadPath := filepath.Join(t.TempDir(), "postgres.tar")
		if err := os.WriteFile(payloadPath, []byte("postgres-volume"), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
				if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
					return system.CommandResult{ExitCode: 0}, nil
				}
				t.Fatal("snapshot restore should stop before removing volumes when verbose output fails")
				return system.CommandResult{}, nil
			}
		})

		cmd := &cobra.Command{Use: "snapshot"}
		cmd.SetOut(&failingWriteBuffer{failAfter: 1})

		manifest := snapshotManifest{
			Version:   1,
			StackName: cfg.Stack.Name,
			Volumes: []snapshotVolumeRecord{{
				Service:    "postgres",
				SourceName: "postgres_data",
				Archive:    "volumes/postgres.tar",
			}},
		}

		err := restoreSnapshotArchive(cmd, specs, manifest, map[string]string{"volumes/postgres.tar": payloadPath})
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected verbose remove write failure, got %v", err)
		}
	})
}
