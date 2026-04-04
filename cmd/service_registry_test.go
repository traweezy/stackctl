package cmd

import (
	"context"
	"slices"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func definitionKeys(definitions []serviceDefinition) []string {
	keys := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		keys = append(keys, definition.Key)
	}
	return keys
}

func containerMapForDefinitions(cfg configpkg.Config, definitions []serviceDefinition) map[string]system.Container {
	containers := make(map[string]system.Container, len(definitions))
	for _, definition := range definitions {
		if definition.ContainerName == nil || definition.PrimaryPort == nil {
			continue
		}
		containerName := definition.ContainerName(cfg)
		containers[containerName] = system.Container{
			ID:     definition.Key + "-id",
			Image:  definition.Key + ":latest",
			Names:  []string{containerName},
			Status: "Up 5 minutes",
			State:  "running",
			Ports: []system.ContainerPort{{
				HostPort:      definition.PrimaryPort(cfg),
				ContainerPort: definition.DefaultInternalPort,
				Protocol:      "tcp",
			}},
			CreatedAt: "now",
		}
	}
	return containers
}

func TestServiceRegistryFiltersLookupsAndNames(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.Setup.IncludeNATS = false
	cfg.Setup.IncludeSeaweedFS = false
	cfg.Setup.IncludeMeilisearch = false
	cfg.Setup.IncludePgAdmin = false
	cfg.Setup.IncludeCockpit = true
	cfg.ApplyDerivedFields()

	enabled := definitionKeys(enabledServiceDefinitions(cfg))
	for _, key := range []string{"postgres", "redis", "cockpit"} {
		if !slices.Contains(enabled, key) {
			t.Fatalf("expected enabled definitions to include %q, got %v", key, enabled)
		}
	}
	for _, key := range []string{"nats", "seaweedfs", "meilisearch", "pgadmin"} {
		if slices.Contains(enabled, key) {
			t.Fatalf("expected enabled definitions to exclude %q, got %v", key, enabled)
		}
	}

	stackOnly := definitionKeys(enabledStackServiceDefinitions(cfg))
	if slices.Contains(stackOnly, "cockpit") {
		t.Fatalf("expected stack-only definitions to exclude cockpit, got %v", stackOnly)
	}
	if !slices.Contains(stackOnly, "postgres") || !slices.Contains(stackOnly, "redis") {
		t.Fatalf("expected stack-only definitions to include postgres and redis, got %v", stackOnly)
	}

	waitable := definitionKeys(waitableServiceDefinitions(cfg))
	if slices.Contains(waitable, "cockpit") || slices.Contains(waitable, "pgadmin") {
		t.Fatalf("expected waitable services to exclude cockpit and pgadmin, got %v", waitable)
	}

	postgres, ok := serviceDefinitionByAlias("PG")
	if !ok || postgres.Key != "postgres" {
		t.Fatalf("expected postgres alias lookup to resolve, got %+v ok=%v", postgres, ok)
	}

	seaweed, ok := serviceDefinitionByAlias("Seaweed")
	if !ok || seaweed.Key != "seaweedfs" {
		t.Fatalf("expected seaweed alias lookup to resolve, got %+v ok=%v", seaweed, ok)
	}

	cockpit, ok := serviceDefinitionByKey("cockpit")
	if !ok || cockpit.Kind != serviceKindHostTool {
		t.Fatalf("expected cockpit key lookup to resolve host tool, got %+v ok=%v", cockpit, ok)
	}

	if _, ok := serviceDefinitionByAlias("unknown"); ok {
		t.Fatal("expected unknown alias lookup to fail")
	}
	if _, ok := serviceDefinitionByKey("unknown"); ok {
		t.Fatal("expected unknown key lookup to fail")
	}

	for _, fragment := range []string{"postgres", "redis", "pgadmin"} {
		if !strings.Contains(validStackServiceNames(), fragment) {
			t.Fatalf("expected valid stack service names to include %q", fragment)
		}
	}
	for _, fragment := range []string{"postgres", "meilisearch-api-key", "cockpit"} {
		if !strings.Contains(validCopyTargetNames(), fragment) {
			t.Fatalf("expected valid copy target names to include %q", fragment)
		}
	}
	for _, fragment := range []string{"postgres", "redis", "cockpit"} {
		if !strings.Contains(validEnvTargetNames(), fragment) {
			t.Fatalf("expected valid env target names to include %q", fragment)
		}
	}
}

func TestServiceRegistryBuildsRuntimeEntriesAndCopyTargets(t *testing.T) {
	withTestDeps(t, func(d *commandDeps) {
		d.cockpitStatus = func(context.Context) system.CockpitState {
			return system.CockpitState{Installed: true, Active: true, State: "active"}
		}
		d.portListening = func(port int) bool { return port > 0 }
		d.portInUse = func(int) (bool, error) { return false, nil }
	})

	cfg := configpkg.DefaultForStack("staging")
	cfg.Setup.IncludeNATS = true
	cfg.Setup.IncludeSeaweedFS = true
	cfg.Setup.IncludeMeilisearch = true
	cfg.Setup.IncludePgAdmin = true
	cfg.Setup.IncludeCockpit = true
	cfg.Connection.RedisPassword = "redis-default"
	cfg.Connection.RedisACLUsername = "cache"
	cfg.Connection.RedisACLPassword = "cache-secret"
	cfg.ApplyDerivedFields()

	definitions := enabledServiceDefinitions(cfg)
	containers := containerMapForDefinitions(cfg, definitions)

	for _, definition := range definitions {
		runtime := runtimeServiceForDefinition(context.Background(), cfg, definition, containers)
		if runtime.Name != definition.Key {
			t.Fatalf("runtime name mismatch for %q: %+v", definition.Key, runtime)
		}
		if runtime.DisplayName == "" {
			t.Fatalf("expected runtime display name for %q", definition.Key)
		}
		if len(definition.EnvEntries(cfg)) == 0 {
			t.Fatalf("expected env entries for %q", definition.Key)
		}
		if len(definition.ConnectionEntries(cfg)) == 0 {
			t.Fatalf("expected connection entries for %q", definition.Key)
		}

		for _, spec := range definition.CopyTargets() {
			primary, ok := copyTargetSpec(cfg, spec.PrimaryAlias)
			if !ok || primary.PrimaryAlias != spec.PrimaryAlias {
				t.Fatalf("expected primary copy target lookup for %q", spec.PrimaryAlias)
			}
			value, err := primary.Resolve(cfg)
			if err != nil {
				t.Fatalf("resolve primary copy target %q: %v", spec.PrimaryAlias, err)
			}
			if strings.TrimSpace(value) == "" {
				t.Fatalf("expected non-empty resolved value for %q", spec.PrimaryAlias)
			}

			for _, alias := range spec.Aliases {
				aliased, ok := copyTargetSpec(cfg, alias)
				if !ok || aliased.PrimaryAlias != spec.PrimaryAlias {
					t.Fatalf("expected alias %q to resolve to %q", alias, spec.PrimaryAlias)
				}
			}
		}
	}
}

func TestServiceRegistryDisabledBranchesAndCopyTargetErrors(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.Setup.IncludeNATS = false
	cfg.Setup.IncludeSeaweedFS = false
	cfg.Setup.IncludeMeilisearch = false
	cfg.Setup.IncludePgAdmin = false
	cfg.Setup.IncludeCockpit = false
	cfg.Connection.RedisPassword = ""
	cfg.Connection.RedisACLUsername = ""
	cfg.Connection.RedisACLPassword = ""
	cfg.ApplyDerivedFields()

	for _, definition := range serviceDefinitions() {
		switch definition.Key {
		case "nats", "seaweedfs", "meilisearch", "pgadmin":
			if len(definition.ConnectionEntries(cfg)) != 0 {
				t.Fatalf("expected disabled %q connection entries to be empty", definition.Key)
			}
		}
	}

	testCases := []struct {
		target string
		cfg    configpkg.Config
		substr string
	}{
		{target: "nats", cfg: cfg, substr: "nats is not enabled"},
		{target: "seaweedfs", cfg: cfg, substr: "seaweedfs is not enabled"},
		{target: "meilisearch", cfg: cfg, substr: "meilisearch is not enabled"},
		{target: "cockpit", cfg: cfg, substr: "cockpit is not enabled"},
		{target: "redis-username", cfg: cfg, substr: "redis ACL auth is not enabled"},
		{target: "redis-default-password", cfg: configpkg.DefaultForStack("staging"), substr: "redis default-user password is not configured"},
	}

	pgAdminNoURL := configpkg.DefaultForStack("staging")
	pgAdminNoURL.Ports.PgAdmin = 0
	pgAdminNoURL.URLs.PgAdmin = ""
	pgAdminNoURL.ApplyDerivedFields()
	testCases = append(testCases, struct {
		target string
		cfg    configpkg.Config
		substr string
	}{
		target: "pgadmin",
		cfg:    pgAdminNoURL,
		substr: "pgadmin is not enabled",
	})

	for _, tc := range testCases {
		spec, ok := copyTargetSpec(tc.cfg, tc.target)
		if !ok {
			t.Fatalf("expected copy target %q to exist", tc.target)
		}
		_, err := spec.Resolve(tc.cfg)
		if err == nil || !strings.Contains(err.Error(), tc.substr) {
			t.Fatalf("expected %q resolve error to contain %q, got %v", tc.target, tc.substr, err)
		}
	}

	if _, ok := copyTargetSpec(cfg, "not-a-target"); ok {
		t.Fatal("expected unknown copy target lookup to fail")
	}
}
