package cmd

import (
	configpkg "github.com/traweezy/stackctl/internal/config"
)

var (
	benchmarkCountSink  int
	benchmarkStringSink string
)

func benchmarkCommandConfig() configpkg.Config {
	cfg := configpkg.DefaultForStack("perf-stack")
	cfg.Setup.IncludeSeaweedFS = true
	cfg.Setup.IncludeMeilisearch = true
	cfg.Setup.IncludeCockpit = true
	cfg.Setup.IncludePgAdmin = true
	cfg.Connection.RedisACLUsername = "bench"
	cfg.Connection.RedisACLPassword = "bench-secret"
	cfg.ApplyDerivedFields()
	return cfg
}

func benchmarkSelectedStackServices() []string {
	return []string{
		"postgres",
		"redis",
		"nats",
		"seaweedfs",
		"meilisearch",
		"pgadmin",
	}
}

func benchmarkSnapshotManifest(specs []persistentVolumeSpec) snapshotManifest {
	manifest := snapshotManifest{
		Version:   1,
		StackName: "restore-target",
		Volumes:   make([]snapshotVolumeRecord, 0, len(specs)),
	}

	for idx := len(specs) - 1; idx >= 0; idx-- {
		spec := specs[idx]
		manifest.Volumes = append(manifest.Volumes, snapshotVolumeRecord{
			Service:    spec.ServiceKey,
			SourceName: spec.VolumeName,
			Archive:    spec.ArchiveEntry,
		})
	}

	return manifest
}
