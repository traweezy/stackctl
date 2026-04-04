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
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	doctorpkg "github.com/traweezy/stackctl/internal/doctor"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestDoctorFixPropagatesConfigPathAndScaffoldErrors(t *testing.T) {
	t.Run("config path error", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(), nil
			}
			d.configFilePath = func() (string, error) {
				return "", errors.New("config path unavailable")
			}
		})

		_, _, err := executeRoot(t, "doctor", "--fix", "--yes")
		if err == nil || !strings.Contains(err.Error(), "config path unavailable") {
			t.Fatalf("unexpected doctor --fix error: %v", err)
		}
	})

	t.Run("managed scaffold check error", func(t *testing.T) {
		cfg := configpkg.Default()

		withTestDeps(t, func(d *commandDeps) {
			d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
				return newReport(), nil
			}
			d.loadConfig = func(string) (configpkg.Config, error) {
				return cfg, nil
			}
			d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
				return false, errors.New("scaffold inspection failed")
			}
		})

		_, _, err := executeRoot(t, "doctor", "--fix", "--yes")
		if err == nil || !strings.Contains(err.Error(), "scaffold inspection failed") {
			t.Fatalf("unexpected doctor --fix error: %v", err)
		}
	})
}

func TestDoctorFixUsesDefaultConfigAndPodmanMachineFlow(t *testing.T) {
	var (
		doctorRuns int
		prepared   bool
	)

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.System.PackageManager = "brew"
		cfg.Setup.IncludeCockpit = false
		cfg.Setup.InstallCockpit = false

		d.defaultConfig = func() configpkg.Config { return cfg }
		d.loadConfig = func(string) (configpkg.Config, error) {
			return configpkg.Config{}, configpkg.ErrNotFound
		}
		d.platform = func() system.Platform {
			return system.Platform{
				GOOS:           "darwin",
				PackageManager: "brew",
				ServiceManager: system.ServiceManagerNone,
			}
		}
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			doctorRuns++
			if doctorRuns == 1 {
				return newReport(
					doctorpkg.Check{Status: output.StatusMiss, Message: "podman machine initialized"},
					doctorpkg.Check{Status: output.StatusMiss, Message: "podman machine running"},
				), nil
			}
			return newReport(
				doctorpkg.Check{Status: output.StatusOK, Message: "podman machine initialized"},
				doctorpkg.Check{Status: output.StatusOK, Message: "podman machine running"},
			), nil
		}
		d.preparePodmanMachine = func(context.Context, system.Runner) error {
			prepared = true
			return nil
		}
	})

	stdout, _, err := executeRoot(t, "doctor", "--fix", "--yes")
	if err != nil {
		t.Fatalf("doctor --fix returned error: %v", err)
	}
	if doctorRuns != 2 {
		t.Fatalf("expected doctor to run twice, got %d", doctorRuns)
	}
	if !prepared {
		t.Fatal("expected doctor --fix to prepare the podman machine")
	}
	if !strings.Contains(stdout, "podman machine is initialized and running") {
		t.Fatalf("expected podman machine status message, got:\n%s", stdout)
	}
}

func TestDoctorFixFailsWhenPostFixReportStillHasFailures(t *testing.T) {
	var doctorRuns int

	withTestDeps(t, func(d *commandDeps) {
		cfg := configpkg.Default()
		cfg.System.PackageManager = "apt"

		d.defaultConfig = func() configpkg.Config { return cfg }
		d.loadConfig = func(string) (configpkg.Config, error) {
			return cfg, nil
		}
		d.platform = func() system.Platform {
			return system.Platform{
				GOOS:           "linux",
				PackageManager: "apt",
				ServiceManager: system.ServiceManagerSystemd,
			}
		}
		d.commandExists = func(name string) bool { return name == "apt-get" }
		d.runDoctor = func(context.Context) (doctorpkg.Report, error) {
			doctorRuns++
			return newReport(
				doctorpkg.Check{Status: output.StatusMiss, Message: "podman installed"},
			), nil
		}
		d.installPackages = func(context.Context, system.Runner, string, []system.Requirement) ([]string, error) {
			return []string{"podman"}, nil
		}
	})

	_, _, err := executeRoot(t, "doctor", "--fix", "--yes")
	if err == nil || !strings.Contains(err.Error(), "doctor still found issues that need attention") {
		t.Fatalf("unexpected doctor --fix error: %v", err)
	}
	if doctorRuns != 2 {
		t.Fatalf("expected doctor to run twice, got %d", doctorRuns)
	}
}

func TestRenderMarkdownBlockHonorsQuietAndRendersContent(t *testing.T) {
	t.Run("quiet", func(t *testing.T) {
		original := rootOutput
		rootOutput.Quiet = true
		t.Cleanup(func() { rootOutput = original })

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		if err := renderMarkdownBlock(cmd, "## Suggested actions\n\n- Review the stack."); err != nil {
			t.Fatalf("renderMarkdownBlock returned error: %v", err)
		}
		if stdout.Len() != 0 {
			t.Fatalf("expected quiet markdown render to stay silent, got %q", stdout.String())
		}
	})

	t.Run("default output", func(t *testing.T) {
		original := rootOutput
		rootOutput.Quiet = false
		t.Cleanup(func() { rootOutput = original })

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		if err := renderMarkdownBlock(cmd, "## Suggested actions\n\n- Review the stack."); err != nil {
			t.Fatalf("renderMarkdownBlock returned error: %v", err)
		}
		rendered := stdout.String()
		if !strings.Contains(rendered, "Suggested actions") || !strings.Contains(rendered, "Review the stack.") {
			t.Fatalf("unexpected markdown render output:\n%s", rendered)
		}
	})
}

func TestReadSnapshotArchiveReportsMissingArchivesAndPathCollisions(t *testing.T) {
	t.Run("missing archive", func(t *testing.T) {
		_, _, cleanup, err := readSnapshotArchive(filepath.Join(t.TempDir(), "missing.tar"))
		if cleanup != nil {
			defer cleanup()
		}
		if err == nil || !strings.Contains(err.Error(), "open snapshot archive") {
			t.Fatalf("unexpected missing-archive error: %v", err)
		}
	})

	t.Run("directory collision while extracting", func(t *testing.T) {
		dir := t.TempDir()
		archivePath := filepath.Join(dir, "collision.tar")

		file, err := os.Create(archivePath)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		writer := tar.NewWriter(file)
		manifestData, err := json.MarshalIndent(snapshotManifest{
			Version:   1,
			StackName: "dev-stack",
		}, "", "  ")
		if err != nil {
			t.Fatalf("MarshalIndent returned error: %v", err)
		}
		if err := writeTarEntry(writer, "manifest.json", manifestData); err != nil {
			t.Fatalf("write manifest entry: %v", err)
		}
		if err := writeTarEntry(writer, "volumes", []byte("not-a-directory")); err != nil {
			t.Fatalf("write blocking file entry: %v", err)
		}
		if err := writeTarEntry(writer, "volumes/redis.tar", []byte("payload")); err != nil {
			t.Fatalf("write nested payload entry: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}

		_, _, cleanup, err := readSnapshotArchive(archivePath)
		if cleanup != nil {
			defer cleanup()
		}
		if err == nil || !strings.Contains(err.Error(), "create extracted snapshot dir volumes") {
			t.Fatalf("unexpected directory-collision error: %v", err)
		}
	})
}
