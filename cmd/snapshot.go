package cmd

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

type snapshotManifest struct {
	Version   int                    `json:"version"`
	StackName string                 `json:"stack_name"`
	Volumes   []snapshotVolumeRecord `json:"volumes"`
}

type snapshotVolumeRecord struct {
	Service    string `json:"service"`
	SourceName string `json:"source_name"`
	Archive    string `json:"archive"`
}

type persistentVolumeSpec struct {
	ServiceKey   string
	DisplayName  string
	VolumeName   string
	ArchiveEntry string
}

func newSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "snapshot",
		Short:             "Save or restore managed service volumes",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
	}

	cmd.AddCommand(newSnapshotSaveCmd())
	cmd.AddCommand(newSnapshotRestoreCmd())

	return cmd
}

func newSnapshotSaveCmd() *cobra.Command {
	var stop bool

	cmd := &cobra.Command{
		Use:   "save <archive-path>",
		Short: "Save a managed stack volume snapshot",
		Example: "  stackctl snapshot save local-stack.tar\n" +
			"  stackctl snapshot save local-stack.tar --stop",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadManagedSnapshotConfig(cmd)
			if err != nil {
				return err
			}

			specs := persistentVolumeSpecs(cfg)
			if len(specs) == 0 {
				return errors.New("no persistent managed service volumes are configured in this stack")
			}

			if err := stopStackForSnapshotIfNeeded(cmd, cfg, stop, false); err != nil {
				return err
			}

			if err := statusLine(cmd, output.StatusInfo, fmt.Sprintf("saving snapshot to %s...", args[0])); err != nil {
				return err
			}
			if err := saveSnapshotArchive(cmd, cfg, specs, args[0]); err != nil {
				return err
			}
			return statusLine(cmd, output.StatusOK, fmt.Sprintf("saved snapshot to %s", args[0]))
		},
	}

	cmd.Flags().BoolVar(&stop, "stop", false, "Stop the running stack before exporting managed volumes")

	return cmd
}

func newSnapshotRestoreCmd() *cobra.Command {
	var stop bool
	var force bool

	cmd := &cobra.Command{
		Use:   "restore <archive-path>",
		Short: "Restore a managed stack volume snapshot",
		Example: "  stackctl snapshot restore local-stack.tar --force\n" +
			"  stackctl snapshot restore local-stack.tar --stop --force",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadManagedSnapshotConfig(cmd)
			if err != nil {
				return err
			}

			if !force {
				ok, err := confirmWithPrompt(cmd, "This will replace the managed stack volumes. Continue?", false)
				if err != nil {
					return fmt.Errorf("snapshot restore confirmation required; rerun with --force")
				}
				if !ok {
					return userCancelled(cmd, "snapshot restore cancelled")
				}
			}

			specs := persistentVolumeSpecs(cfg)
			if len(specs) == 0 {
				return errors.New("no persistent managed service volumes are configured in this stack")
			}

			manifest, extracted, cleanup, err := readSnapshotArchive(args[0])
			if err != nil {
				return err
			}
			defer cleanup()

			if err := validateSnapshotManifest(cfg, specs, manifest); err != nil {
				return err
			}
			if err := validateSnapshotArchivePayloads(manifest, extracted); err != nil {
				return err
			}

			if err := stopStackForSnapshotIfNeeded(cmd, cfg, stop, true); err != nil {
				return err
			}

			if err := statusLine(cmd, output.StatusReset, fmt.Sprintf("restoring snapshot from %s...", args[0])); err != nil {
				return err
			}
			if err := restoreSnapshotArchive(cmd, specs, manifest, extracted); err != nil {
				return err
			}
			return statusLine(cmd, output.StatusOK, fmt.Sprintf("restored snapshot from %s", args[0]))
		},
	}

	cmd.Flags().BoolVar(&stop, "stop", false, "Stop the running stack before replacing managed volumes")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation before replacing managed volumes")

	return cmd
}

func loadManagedSnapshotConfig(cmd *cobra.Command) (configpkg.Config, error) {
	cfg, err := loadRuntimeConfig(cmd, false)
	if err != nil {
		return configpkg.Config{}, err
	}
	if !cfg.Stack.Managed {
		return configpkg.Config{}, errors.New("snapshot commands require a managed stack")
	}
	if err := syncManagedScaffoldIfNeeded(cmd, cfg); err != nil {
		return configpkg.Config{}, err
	}
	if !deps.commandExists("podman") {
		return configpkg.Config{}, errors.New("podman is not installed; run `stackctl setup --install` or install it manually")
	}
	return cfg, nil
}

func persistentVolumeSpecs(cfg configpkg.Config) []persistentVolumeSpec {
	specs := make([]persistentVolumeSpec, 0, 5)
	appendSpec := func(serviceKey, displayName, volumeName string) {
		if strings.TrimSpace(volumeName) == "" {
			return
		}
		specs = append(specs, persistentVolumeSpec{
			ServiceKey:   serviceKey,
			DisplayName:  displayName,
			VolumeName:   volumeName,
			ArchiveEntry: filepath.ToSlash(filepath.Join("volumes", serviceKey+".tar")),
		})
	}

	if cfg.PostgresEnabled() {
		appendSpec("postgres", "Postgres", cfg.Services.Postgres.DataVolume)
	}
	if cfg.RedisEnabled() {
		appendSpec("redis", "Redis", cfg.Services.Redis.DataVolume)
	}
	if cfg.SeaweedFSEnabled() {
		appendSpec("seaweedfs", "SeaweedFS", cfg.Services.SeaweedFS.DataVolume)
	}
	if cfg.MeilisearchEnabled() {
		appendSpec("meilisearch", "Meilisearch", cfg.Services.Meilisearch.DataVolume)
	}
	if cfg.PgAdminEnabled() {
		appendSpec("pgadmin", "pgAdmin", cfg.Services.PgAdmin.DataVolume)
	}

	return specs
}

func stopStackForSnapshotIfNeeded(cmd *cobra.Command, cfg configpkg.Config, stopRequested bool, destructive bool) error {
	running, err := runningStackServices(context.Background(), cfg)
	if err != nil {
		return err
	}
	if len(running) == 0 {
		return nil
	}
	if !stopRequested {
		action := "save a snapshot"
		if destructive {
			action = "restore a snapshot"
		}
		return fmt.Errorf("the stack is running (%s); stop it first or rerun with --stop to %s", strings.Join(running, ", "), action)
	}

	if err := verboseLine(cmd, fmt.Sprintf("Stopping running stack services: %s", strings.Join(running, ", "))); err != nil {
		return err
	}
	if err := ensureComposeRuntime(cmd, cfg); err != nil {
		return err
	}
	return composeDownAndWait(context.Background(), runnerFor(cmd), cfg, false)
}

func saveSnapshotArchive(cmd *cobra.Command, cfg configpkg.Config, specs []persistentVolumeSpec, archivePath string) error {
	tempDir, err := os.MkdirTemp("", "stackctl-snapshot-save-*")
	if err != nil {
		return fmt.Errorf("create snapshot temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	manifest := snapshotManifest{
		Version:   1,
		StackName: cfg.Stack.Name,
		Volumes:   make([]snapshotVolumeRecord, 0, len(specs)),
	}
	exportedFiles := make(map[string]string, len(specs))
	for _, spec := range specs {
		exists, err := podmanVolumeExists(ctx, spec.VolumeName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("managed volume %s for %s does not exist", spec.VolumeName, spec.DisplayName)
		}

		exportPath := filepath.Join(tempDir, spec.ServiceKey+".tar")
		if err := verboseLine(cmd, fmt.Sprintf("Exporting %s volume %s", spec.DisplayName, spec.VolumeName)); err != nil {
			return err
		}
		if err := runnerFor(cmd).Run(ctx, "", "podman", "volume", "export", spec.VolumeName, "--output", exportPath); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		exportedFiles[spec.ArchiveEntry] = exportPath
		manifest.Volumes = append(manifest.Volumes, snapshotVolumeRecord{
			Service:    spec.ServiceKey,
			SourceName: spec.VolumeName,
			Archive:    spec.ArchiveEntry,
		})
	}

	return writeSnapshotArchive(archivePath, manifest, exportedFiles)
}

func restoreSnapshotArchive(cmd *cobra.Command, specs []persistentVolumeSpec, manifest snapshotManifest, extracted map[string]string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	for _, spec := range specs {
		exists, err := podmanVolumeExists(ctx, spec.VolumeName)
		if err != nil {
			return err
		}
		if exists {
			if err := verboseLine(cmd, fmt.Sprintf("Removing existing volume %s", spec.VolumeName)); err != nil {
				return err
			}
			if err := runnerFor(cmd).Run(ctx, "", "podman", "volume", "rm", spec.VolumeName); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		}
		if err := runnerFor(cmd).Run(ctx, "", "podman", "volume", "create", spec.VolumeName); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}

	specByService := make(map[string]persistentVolumeSpec, len(specs))
	for _, spec := range specs {
		specByService[spec.ServiceKey] = spec
	}

	for _, entry := range manifest.Volumes {
		spec := specByService[entry.Service]
		sourcePath := extracted[entry.Archive]
		if err := verboseLine(cmd, fmt.Sprintf("Importing %s volume %s", spec.DisplayName, spec.VolumeName)); err != nil {
			return err
		}
		if err := runnerFor(cmd).Run(ctx, "", "podman", "volume", "import", spec.VolumeName, sourcePath); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}

	return nil
}

func podmanVolumeExists(ctx context.Context, name string) (bool, error) {
	result, err := deps.captureResult(ctx, "", "podman", "volume", "exists", name)
	if err != nil {
		return false, err
	}
	switch result.ExitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		if detail == "" {
			detail = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		return false, fmt.Errorf("check volume %s: %s", name, detail)
	}
}

func writeSnapshotArchive(path string, manifest snapshotManifest, files map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create snapshot archive %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	writer := tar.NewWriter(file)
	defer func() { _ = writer.Close() }()

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot manifest: %w", err)
	}
	if err := writeTarEntry(writer, "manifest.json", manifestData); err != nil {
		return fmt.Errorf("write snapshot manifest: %w", err)
	}

	for _, entry := range manifest.Volumes {
		sourcePath := files[entry.Archive]
		if err := writeTarFile(writer, entry.Archive, sourcePath); err != nil {
			return fmt.Errorf("write snapshot entry %s: %w", entry.Archive, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("finalize snapshot archive %s: %w", path, err)
	}
	return file.Close()
}

func readSnapshotArchive(path string) (snapshotManifest, map[string]string, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return snapshotManifest{}, nil, nil, fmt.Errorf("open snapshot archive %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	tempDir, err := os.MkdirTemp("", "stackctl-snapshot-restore-*")
	if err != nil {
		return snapshotManifest{}, nil, nil, fmt.Errorf("create snapshot extraction dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	reader := tar.NewReader(file)
	manifest := snapshotManifest{}
	extracted := make(map[string]string)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return snapshotManifest{}, nil, nil, fmt.Errorf("read snapshot archive %s: %w", path, err)
		}
		if header.FileInfo().IsDir() {
			continue
		}

		switch filepath.ToSlash(header.Name) {
		case "manifest.json":
			data, err := io.ReadAll(reader)
			if err != nil {
				cleanup()
				return snapshotManifest{}, nil, nil, fmt.Errorf("read snapshot manifest: %w", err)
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				cleanup()
				return snapshotManifest{}, nil, nil, fmt.Errorf("parse snapshot manifest: %w", err)
			}
		default:
			targetPath := filepath.Join(tempDir, filepath.Base(header.Name))
			targetFile, err := os.Create(targetPath)
			if err != nil {
				cleanup()
				return snapshotManifest{}, nil, nil, fmt.Errorf("create extracted snapshot file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(targetFile, reader); err != nil {
				_ = targetFile.Close()
				cleanup()
				return snapshotManifest{}, nil, nil, fmt.Errorf("extract snapshot entry %s: %w", header.Name, err)
			}
			if err := targetFile.Close(); err != nil {
				cleanup()
				return snapshotManifest{}, nil, nil, fmt.Errorf("close extracted snapshot file %s: %w", targetPath, err)
			}
			extracted[filepath.ToSlash(header.Name)] = targetPath
		}
	}

	if manifest.Version == 0 {
		cleanup()
		return snapshotManifest{}, nil, nil, errors.New("snapshot archive is missing manifest metadata")
	}
	return manifest, extracted, cleanup, nil
}

func validateSnapshotManifest(cfg configpkg.Config, specs []persistentVolumeSpec, manifest snapshotManifest) error {
	if manifest.Version != 1 {
		return fmt.Errorf("unsupported snapshot archive version %d", manifest.Version)
	}

	expected := make([]string, 0, len(specs))
	for _, spec := range specs {
		expected = append(expected, spec.ServiceKey)
	}
	slices.Sort(expected)

	archived := make([]string, 0, len(manifest.Volumes))
	for _, entry := range manifest.Volumes {
		archived = append(archived, entry.Service)
	}
	slices.Sort(archived)

	if !slices.Equal(expected, archived) {
		return fmt.Errorf(
			"snapshot services do not match the current stack; archive has [%s], current stack expects [%s]",
			strings.Join(archived, ", "),
			strings.Join(expected, ", "),
		)
	}

	archiveSet := make(map[string]struct{}, len(manifest.Volumes))
	for _, entry := range manifest.Volumes {
		if strings.TrimSpace(entry.Archive) == "" {
			return fmt.Errorf("snapshot entry for %s is missing its archive path", entry.Service)
		}
		archiveSet[entry.Archive] = struct{}{}
	}
	if len(archiveSet) != len(manifest.Volumes) {
		return errors.New("snapshot archive manifest contains duplicate volume entries")
	}

	if cfg.Stack.Name != manifest.StackName {
		// Cross-stack restores are allowed as long as the service set matches.
		return nil
	}
	return nil
}

func validateSnapshotArchivePayloads(manifest snapshotManifest, extracted map[string]string) error {
	for _, entry := range manifest.Volumes {
		path := strings.TrimSpace(extracted[entry.Archive])
		if path == "" {
			return fmt.Errorf("snapshot archive is missing payload %s for %s", entry.Archive, entry.Service)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("inspect snapshot payload %s for %s: %w", entry.Archive, entry.Service, err)
		}
		if info.IsDir() {
			return fmt.Errorf("snapshot payload %s for %s is a directory", entry.Archive, entry.Service)
		}
	}

	return nil
}

func writeTarEntry(writer *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name: name,
		Mode: 0o600,
		Size: int64(len(data)),
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

func writeTarFile(writer *tar.Writer, entryName, sourcePath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	header := &tar.Header{
		Name: entryName,
		Mode: 0o600,
		Size: info.Size(),
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.Copy(writer, sourceFile)
	return err
}
