package cmd

import (
	"bytes"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestRuntimeEndpointHelpersPreferConfiguredURLs(t *testing.T) {
	cfg := configpkg.Default()
	cfg.Connection.Host = "devbox"
	cfg.Ports.SeaweedFS = 9333
	cfg.Ports.Meilisearch = 7700
	cfg.URLs.SeaweedFS = ""
	cfg.URLs.Meilisearch = ""

	if got := seaweedFSEndpoint(cfg); got != "http://devbox:9333" {
		t.Fatalf("unexpected SeaweedFS fallback endpoint: %q", got)
	}
	if got := meilisearchURL(cfg); got != "http://devbox:7700" {
		t.Fatalf("unexpected Meilisearch fallback endpoint: %q", got)
	}

	cfg.URLs.SeaweedFS = "https://storage.example.test"
	cfg.URLs.Meilisearch = "https://search.example.test"

	if got := seaweedFSEndpoint(cfg); got != "https://storage.example.test" {
		t.Fatalf("expected explicit SeaweedFS URL, got %q", got)
	}
	if got := meilisearchURL(cfg); got != "https://search.example.test" {
		t.Fatalf("expected explicit Meilisearch URL, got %q", got)
	}
}

func TestRuntimeFormattingHelpers(t *testing.T) {
	tests := []struct {
		name         string
		externalPort int
		internalPort int
		want         string
	}{
		{name: "both ports", externalPort: 15432, internalPort: 5432, want: "15432 -> 5432"},
		{name: "missing internal", externalPort: 15432, want: "15432 -> unknown"},
		{name: "missing external", internalPort: 5432, want: "unknown -> 5432"},
		{name: "missing both", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatServicePort(tt.externalPort, tt.internalPort); got != tt.want {
				t.Fatalf("unexpected formatted port: got %q want %q", got, tt.want)
			}
		})
	}

	if got := missingPortLabel("postgres port listening"); got != "postgres port not listening" {
		t.Fatalf("unexpected missing port label: %q", got)
	}
	if got := missingPortLabel("cockpit url"); got != "cockpit url" {
		t.Fatalf("unexpected unchanged label: %q", got)
	}
}

func TestRuntimeEnvGroupHelpers(t *testing.T) {
	cfg := configpkg.Default()

	if got := envGroupForDefinition(cfg, serviceDefinition{DisplayName: "Postgres"}); got.Title != "" || len(got.Entries) != 0 {
		t.Fatalf("expected empty env group for nil env entries, got %+v", got)
	}

	definition := serviceDefinition{
		DisplayName: "Postgres",
		EnvEntries: func(configpkg.Config) []envEntry {
			return []envEntry{
				{Name: "DATABASE_URL", Value: "postgres://app"},
				{Name: "PGUSER", Value: "app"},
			}
		},
	}

	group := envGroupForDefinition(cfg, definition)
	if group.Title != "Postgres" || len(group.Entries) != 2 {
		t.Fatalf("unexpected env group: %+v", group)
	}

	var rendered bytes.Buffer
	err := writeEnvGroups(&rendered, []envGroup{
		{},
		group,
		{
			Title: "Redis",
			Entries: []envEntry{
				{Name: "REDIS_URL", Value: "redis://localhost:6379"},
			},
		},
	}, true)
	if err != nil {
		t.Fatalf("writeEnvGroups returned error: %v", err)
	}

	output := rendered.String()
	for _, fragment := range []string{
		"# Postgres",
		"export DATABASE_URL='postgres://app'",
		"export PGUSER='app'",
		"# Redis",
		"export REDIS_URL='redis://localhost:6379'",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("rendered env groups missing %q:\n%s", fragment, output)
		}
	}
	if strings.Contains(output, "# \n") {
		t.Fatalf("rendered env groups should skip empty sections:\n%s", output)
	}
}
