package cmd

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

type signalOnWriteBuffer struct {
	bytes.Buffer
}

func (w *signalOnWriteBuffer) Write(p []byte) (int, error) {
	_, _ = w.Buffer.Write(p)
	_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
	time.Sleep(20 * time.Millisecond)
	return len(p), nil
}

func TestSnapshotAdditionalCoverageBatch(t *testing.T) {
	t.Run("snapshot save and restore surface managed-config load failures", func(t *testing.T) {
		cases := []struct {
			name string
			args []string
		}{
			{name: "save", args: []string{"snapshot", "save", filepath.Join(t.TempDir(), "snapshot.tar")}},
			{name: "restore", args: []string{"snapshot", "restore", filepath.Join(t.TempDir(), "snapshot.tar"), "--force"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				withTestDeps(t, func(d *commandDeps) {
					d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("load failed") }
				})

				_, _, err := executeRoot(t, tc.args...)
				if err == nil || !strings.Contains(err.Error(), "load failed") {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("snapshot restore propagates archive-open failures after config checks", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "snapshot", "restore", filepath.Join(t.TempDir(), "missing", "snapshot.tar"), "--force")
		if err == nil || !strings.Contains(err.Error(), "open snapshot archive") {
			t.Fatalf("unexpected archive-open error: %v", err)
		}
	})

	t.Run("stopStackForSnapshotIfNeeded covers detection and verbose failure paths", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		t.Run("running service detection failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{}, errors.New("ps failed")
				}
			})

			err := stopStackForSnapshotIfNeeded(&cobra.Command{Use: "snapshot"}, cfg, true, false)
			if err == nil || !strings.Contains(err.Error(), "ps failed") {
				t.Fatalf("unexpected detection error: %v", err)
			}
		})

		original := rootOutput
		rootOutput.Verbose = true
		t.Cleanup(func() { rootOutput = original })

		t.Run("stop notice write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
				}
			})

			cmd := &cobra.Command{Use: "snapshot"}
			cmd.SetOut(&failingWriteBuffer{failAfter: 1})
			if err := stopStackForSnapshotIfNeeded(cmd, cfg, true, false); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected stop notice write failure, got %v", err)
			}
		})

		t.Run("compose file verbose write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
				}
				d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
					t.Fatal("composeDown should not run when verbose compose output fails")
					return nil
				}
			})

			cmd := &cobra.Command{Use: "snapshot"}
			cmd.SetOut(&substringWriteErrorWriter{target: "Using compose file"})
			if err := stopStackForSnapshotIfNeeded(cmd, cfg, true, false); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected verbose compose write failure, got %v", err)
			}
		})
	})

	t.Run("saveSnapshotArchive covers podman exists and signal-cancel export branches", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()
		specs := []persistentVolumeSpec{{
			ServiceKey:   "postgres",
			DisplayName:  "Postgres",
			VolumeName:   "postgres_data",
			ArchiveEntry: "volumes/postgres.tar",
		}}

		t.Run("podman volume exists failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(_ context.Context, _ string, _ string, _ ...string) (system.CommandResult, error) {
					return system.CommandResult{}, errors.New("exists failed")
				}
			})

			err := saveSnapshotArchive(&cobra.Command{Use: "snapshot"}, cfg, specs, filepath.Join(t.TempDir(), "snapshot.tar"))
			if err == nil || !strings.Contains(err.Error(), "exists failed") {
				t.Fatalf("unexpected exists failure: %v", err)
			}
		})

		t.Run("signal cancellation suppresses export errors", func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "podman.log")
			writeFakePodman(t, dir, logPath)
			t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
					if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
						_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
						time.Sleep(20 * time.Millisecond)
						return system.CommandResult{ExitCode: 0}, nil
					}
					return system.CommandResult{Stdout: "[]"}, nil
				}
			})

			if err := saveSnapshotArchive(&cobra.Command{Use: "snapshot"}, cfg, specs, filepath.Join(t.TempDir(), "snapshot.tar")); err != nil {
				t.Fatalf("expected export cancellation to return nil, got %v", err)
			}
		})
	})

	t.Run("restoreSnapshotArchive covers cancel and import notice branches", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "podman.log")
		writeFakePodman(t, dir, logPath)
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

		payloadPath := filepath.Join(dir, "postgres.tar")
		if err := os.WriteFile(payloadPath, []byte("postgres-volume"), 0o644); err != nil {
			t.Fatalf("write snapshot payload: %v", err)
		}

		specs := []persistentVolumeSpec{{
			ServiceKey:   "postgres",
			DisplayName:  "Postgres",
			VolumeName:   "postgres_data",
			ArchiveEntry: "volumes/postgres.tar",
		}}
		manifest := snapshotManifest{
			Version:   1,
			StackName: "dev-stack",
			Volumes: []snapshotVolumeRecord{{
				Service:    "postgres",
				SourceName: "postgres_data",
				Archive:    "volumes/postgres.tar",
			}},
		}
		files := map[string]string{"volumes/postgres.tar": payloadPath}

		t.Run("signal cancellation suppresses remove errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
					if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
						_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
						time.Sleep(20 * time.Millisecond)
						return system.CommandResult{ExitCode: 0}, nil
					}
					return system.CommandResult{Stdout: "[]"}, nil
				}
			})

			if err := restoreSnapshotArchive(&cobra.Command{Use: "snapshot"}, specs, manifest, files); err != nil {
				t.Fatalf("expected remove cancellation to return nil, got %v", err)
			}
		})

		t.Run("signal cancellation suppresses create errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
					if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
						_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
						time.Sleep(20 * time.Millisecond)
						return system.CommandResult{ExitCode: 1}, nil
					}
					return system.CommandResult{Stdout: "[]"}, nil
				}
			})

			if err := restoreSnapshotArchive(&cobra.Command{Use: "snapshot"}, specs, manifest, files); err != nil {
				t.Fatalf("expected create cancellation to return nil, got %v", err)
			}
		})

		original := rootOutput
		rootOutput.Verbose = true
		t.Cleanup(func() { rootOutput = original })

		t.Run("import notice write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
					if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
						return system.CommandResult{ExitCode: 1}, nil
					}
					return system.CommandResult{Stdout: "[]"}, nil
				}
			})

			cmd := &cobra.Command{Use: "snapshot"}
			cmd.SetOut(&substringWriteErrorWriter{target: "Importing Postgres volume"})
			err := restoreSnapshotArchive(cmd, specs, manifest, files)
			if err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected import notice write failure, got %v", err)
			}
		})

		t.Run("signal cancellation suppresses import errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(_ context.Context, _ string, name string, args ...string) (system.CommandResult, error) {
					if name == "podman" && len(args) >= 3 && args[0] == "volume" && args[1] == "exists" {
						return system.CommandResult{ExitCode: 1}, nil
					}
					return system.CommandResult{Stdout: "[]"}, nil
				}
			})

			cmd := &cobra.Command{Use: "snapshot"}
			cmd.SetOut(&signalOnWriteBuffer{})
			if err := restoreSnapshotArchive(cmd, specs, manifest, files); err != nil {
				t.Fatalf("expected import cancellation to return nil, got %v", err)
			}
		})
	})

	t.Run("snapshot archive helpers cover open-root and file-open failures", func(t *testing.T) {
		manifest := snapshotManifest{Version: 1, StackName: "dev-stack"}

		missingRootPath := filepath.Join(t.TempDir(), "missing", "snapshot.tar")
		if err := writeSnapshotArchive(missingRootPath, manifest, nil); err == nil || !strings.Contains(err.Error(), "create snapshot archive") {
			t.Fatalf("expected missing-root write error, got %v", err)
		}

		archiveDir := t.TempDir()
		if err := writeSnapshotArchive(archiveDir, manifest, nil); err == nil || !strings.Contains(err.Error(), "create snapshot archive") {
			t.Fatalf("expected directory write error, got %v", err)
		}

		if _, _, cleanup, err := readSnapshotArchive(missingRootPath); err == nil || !strings.Contains(err.Error(), "open snapshot archive") {
			if cleanup != nil {
				defer cleanup()
			}
			t.Fatalf("expected missing-root read error, got %v", err)
		} else if cleanup != nil {
			defer cleanup()
		}
	})

	t.Run("snapshot archive helpers cover directory entries and tar write failures", func(t *testing.T) {
		dir := t.TempDir()
		archivePath := filepath.Join(dir, "with-dir.tar")
		file, err := os.Create(archivePath)
		if err != nil {
			t.Fatalf("create archive: %v", err)
		}

		writer := tar.NewWriter(file)
		if err := writer.WriteHeader(&tar.Header{Name: "volumes/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
			t.Fatalf("write dir entry: %v", err)
		}
		manifestData, err := json.Marshal(snapshotManifest{Version: 1, StackName: "dev-stack"})
		if err != nil {
			t.Fatalf("marshal manifest: %v", err)
		}
		if err := writeTarEntry(writer, "manifest.json", manifestData); err != nil {
			t.Fatalf("write manifest entry: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close archive writer: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close archive file: %v", err)
		}

		manifest, extracted, cleanup, err := readSnapshotArchive(archivePath)
		if err != nil {
			t.Fatalf("read snapshot archive: %v", err)
		}
		defer cleanup()
		if manifest.Version != 1 || len(extracted) != 0 {
			t.Fatalf("expected directory entries to be skipped, manifest=%+v extracted=%+v", manifest, extracted)
		}

		missingSource := filepath.Join(dir, "missing", "postgres.tar")
		if err := writeTarFile(tar.NewWriter(&bytes.Buffer{}), "volumes/postgres.tar", missingSource); err == nil {
			t.Fatal("expected writeTarFile to fail when the source root is missing")
		}

		payloadPath := filepath.Join(dir, "postgres.tar")
		if err := os.WriteFile(payloadPath, []byte("payload"), 0o600); err != nil {
			t.Fatalf("write payload: %v", err)
		}
		closedWriter := tar.NewWriter(&bytes.Buffer{})
		if err := closedWriter.Close(); err != nil {
			t.Fatalf("close tar writer: %v", err)
		}
		if err := writeTarFile(closedWriter, "volumes/postgres.tar", payloadPath); err == nil {
			t.Fatal("expected writeTarFile to fail after the tar writer has been closed")
		}
	})
}
