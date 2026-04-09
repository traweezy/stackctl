package cmd

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

type copyTargetResolver func(configpkg.Config) (string, error)

func TestRuntimeHelpersAndOutputEdgeCases(t *testing.T) {
	t.Run("ports and run helpers skip definitions without ports", func(t *testing.T) {
		cfg := configpkg.Default()

		originalPortMappingDefinitions := portMappingDefinitionsForConfig
		originalRunSelectedDefinitions := runSelectedStackServiceDefinitions
		t.Cleanup(func() {
			portMappingDefinitionsForConfig = originalPortMappingDefinitions
			runSelectedStackServiceDefinitions = originalRunSelectedDefinitions
		})

		portMappingDefinitionsForConfig = func(configpkg.Config) []serviceDefinition {
			return []serviceDefinition{
				{Key: "skip"},
				{Key: "postgres", DisplayName: "Postgres", PrimaryPort: func(configpkg.Config) int { return 5432 }},
			}
		}
		mappings := configuredPortMappings(cfg)
		if len(mappings) != 1 || mappings[0].Service != "postgres" {
			t.Fatalf("expected configuredPortMappings to skip nil-port definitions, got %+v", mappings)
		}

		runSelectedStackServiceDefinitions = func(configpkg.Config, []string) []serviceDefinition {
			return []serviceDefinition{{Key: "skip"}}
		}
		if err := waitForRunServices(context.Background(), cfg, []string{"skip"}); err != nil {
			t.Fatalf("expected waitForRunServices to skip nil-port definitions, got %v", err)
		}
	})

	t.Run("loadPortMappings skips runtime services that are not in the configured map", func(t *testing.T) {
		cfg := configpkg.Default()
		originalPortMappingDefinitions := portMappingDefinitionsForConfig
		t.Cleanup(func() { portMappingDefinitionsForConfig = originalPortMappingDefinitions })
		portMappingDefinitionsForConfig = func(configpkg.Config) []serviceDefinition {
			return []serviceDefinition{
				{Key: "postgres", DisplayName: "Postgres", PrimaryPort: func(configpkg.Config) int { return 5432 }},
			}
		}

		withTestDeps(t, func(d *commandDeps) {
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: marshalContainersJSON(system.Container{
					Names:  []string{"stack-redis"},
					State:  "running",
					Status: "Up",
					Ports:  []system.ContainerPort{{HostPort: 6379, ContainerPort: 6379}},
				})}, nil
			}
		})

		mappings := loadPortMappings(context.Background(), cfg)
		if len(mappings) != 1 || mappings[0].Service != "postgres" {
			t.Fatalf("expected unmatched runtime services to be ignored, got %+v", mappings)
		}
	})

	t.Run("run command covers env group failures and canceled external commands", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		originalByAlias := runtimeServiceDefinitionByAlias
		originalRunNotifyContext := runNotifyContext
		t.Cleanup(func() {
			runtimeServiceDefinitionByAlias = originalByAlias
			runNotifyContext = originalRunNotifyContext
		})

		callCount := 0
		runtimeServiceDefinitionByAlias = func(name string) (serviceDefinition, bool) {
			callCount++
			if callCount == 1 {
				return serviceDefinition{
					Key:         "postgres",
					Kind:        serviceKindStack,
					Enabled:     func(configpkg.Config) bool { return true },
					PrimaryPort: func(configpkg.Config) int { return 5432 },
				}, true
			}
			return serviceDefinition{}, false
		}
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
		})

		_, _, err := executeRoot(t, "run", "postgres", "--", "printenv")
		if err == nil || !strings.Contains(err.Error(), "invalid env target") {
			t.Fatalf("expected envGroups failure, got %v", err)
		}

		runtimeServiceDefinitionByAlias = originalByAlias
		runNotifyContext = func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, func() {}
		}
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
			}
			d.runExternalCommand = func(context.Context, system.Runner, string, []string) error {
				return errors.New("external failed")
			}
		})
		if _, _, err := executeRoot(t, "run", "postgres", "--no-start", "--", "printenv"); err != nil {
			t.Fatalf("expected canceled run context to suppress external command errors, got %v", err)
		}
	})

	t.Run("runtime helpers skip incomplete definitions", func(t *testing.T) {
		cfg := configpkg.Default()
		originalByAlias := runtimeServiceDefinitionByAlias
		originalStackDefinitions := runtimeEnabledStackServiceDefinitions
		originalDefinitions := runtimeEnabledServiceDefinitions
		t.Cleanup(func() {
			runtimeServiceDefinitionByAlias = originalByAlias
			runtimeEnabledStackServiceDefinitions = originalStackDefinitions
			runtimeEnabledServiceDefinitions = originalDefinitions
		})

		runtimeEnabledStackServiceDefinitions = func(configpkg.Config) []serviceDefinition { return nil }
		runtimeEnabledServiceDefinitions = func(configpkg.Config) []serviceDefinition {
			return []serviceDefinition{
				{Key: "skip-port"},
				{Key: "skip-label", PrimaryPort: func(configpkg.Config) int { return 5432 }},
			}
		}
		lines, err := healthChecks(context.Background(), cfg)
		if err != nil || len(lines) != 0 {
			t.Fatalf("expected healthChecks to skip incomplete definitions, lines=%+v err=%v", lines, err)
		}

		runtimeServiceDefinitionByAlias = func(string) (serviceDefinition, bool) {
			return serviceDefinition{
				Key:        "empty",
				Kind:       serviceKindStack,
				Enabled:    func(configpkg.Config) bool { return true },
				EnvEntries: func(configpkg.Config) []envEntry { return nil },
			}, true
		}
		groups, err := envGroups(cfg, []string{"empty"})
		if err != nil {
			t.Fatalf("envGroups returned error: %v", err)
		}
		if len(groups) != 1 || groups[0].Title != "stackctl" {
			t.Fatalf("expected envGroups to skip empty selected groups, got %+v", groups)
		}
	})

	t.Run("runtime helpers cover defensive definition and marshal branches", func(t *testing.T) {
		originalByAlias := runtimeServiceDefinitionByAlias
		originalStackDefinitions := runtimeEnabledStackServiceDefinitions
		originalDefinitions := runtimeEnabledServiceDefinitions
		originalMarshal := marshalRuntimeJSON
		t.Cleanup(func() {
			runtimeServiceDefinitionByAlias = originalByAlias
			runtimeEnabledStackServiceDefinitions = originalStackDefinitions
			runtimeEnabledServiceDefinitions = originalDefinitions
			marshalRuntimeJSON = originalMarshal
		})

		cfg := configpkg.Default()
		runtimeServiceDefinitionByAlias = func(string) (serviceDefinition, bool) {
			return serviceDefinition{Key: "ghost", Kind: serviceKindStack}, true
		}
		if _, err := serviceContainer(cfg, "ghost"); err == nil || !strings.Contains(err.Error(), "does not define a container name") {
			t.Fatalf("expected missing container-name error, got %v", err)
		}

		runtimeEnabledStackServiceDefinitions = func(configpkg.Config) []serviceDefinition {
			return []serviceDefinition{
				{Key: "skip-container"},
				{Key: "skip-port", ContainerName: func(configpkg.Config) string { return "skip-port" }},
			}
		}
		if names := stackContainerNames(cfg); len(names) != 1 || names[0] != "skip-port" {
			t.Fatalf("unexpected stack container names: %+v", names)
		}
		if services := configuredStackServices(cfg); len(services) != 0 {
			t.Fatalf("expected configured stack services with missing ports to be skipped, got %+v", services)
		}

		runtimeEnabledServiceDefinitions = func(configpkg.Config) []serviceDefinition {
			return []serviceDefinition{
				{Key: "skip-check"},
				{
					Key:               "empty-env",
					ConnectionEntries: nil,
					EnvEntries:        func(configpkg.Config) []envEntry { return nil },
					Enabled:           func(configpkg.Config) bool { return true },
				},
			}
		}
		if got := connectionEntries(cfg); len(got) != 0 {
			t.Fatalf("expected connection entries to skip definitions without entry builders, got %+v", got)
		}
		groups, err := envGroups(cfg, nil)
		if err != nil {
			t.Fatalf("envGroups returned error: %v", err)
		}
		if len(groups) != 1 || groups[0].Title != "stackctl" {
			t.Fatalf("expected envGroups to skip empty groups, got %+v", groups)
		}

		expectedErr := errors.New("marshal failed")
		marshalRuntimeJSON = func(any, string, string) ([]byte, error) { return nil, expectedErr }
		if err := printServicesJSON(&cobra.Command{Use: "services"}, cfg); !errors.Is(err, expectedErr) {
			t.Fatalf("expected printServicesJSON to surface %v, got %v", expectedErr, err)
		}
		if err := printEnvJSON(&cobra.Command{Use: "env"}, cfg, nil); !errors.Is(err, expectedErr) {
			t.Fatalf("expected printEnvJSON to surface %v, got %v", expectedErr, err)
		}
	})

	t.Run("redis copy targets reject disabled and incomplete requests", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.Setup.IncludeRedis = false

		var redisDSN copyTargetResolver
		var redisUsername copyTargetResolver
		var redisDefaultPassword copyTargetResolver
		for _, definition := range serviceDefinitions() {
			if definition.Key != "redis" {
				continue
			}
			for _, target := range definition.CopyTargets() {
				switch target.PrimaryAlias {
				case "redis":
					redisDSN = target.Resolve
				case "redis-username":
					redisUsername = target.Resolve
				case "redis-default-password":
					redisDefaultPassword = target.Resolve
				}
			}
		}

		if _, err := redisDSN(cfg); err == nil || !strings.Contains(err.Error(), "redis is not enabled") {
			t.Fatalf("expected disabled-redis DSN error, got %v", err)
		}
		if _, err := redisUsername(cfg); err == nil || !strings.Contains(err.Error(), "redis is not enabled") {
			t.Fatalf("expected disabled redis username error, got %v", err)
		}
		if _, err := redisDefaultPassword(cfg); err == nil || !strings.Contains(err.Error(), "redis is not enabled") {
			t.Fatalf("expected disabled redis password error, got %v", err)
		}

		cfg = configpkg.Default()
		cfg.Connection.RedisACLUsername = ""
		cfg.Connection.RedisACLPassword = ""
		if _, err := redisUsername(cfg); err == nil || !strings.Contains(err.Error(), "redis ACL auth is not enabled") {
			t.Fatalf("expected missing ACL auth error, got %v", err)
		}
		if _, err := redisDefaultPassword(cfg); err == nil || !strings.Contains(err.Error(), "default-user password is not configured") {
			t.Fatalf("expected missing default-user password error, got %v", err)
		}
	})

	t.Run("status and version surface JSON marshal failures", func(t *testing.T) {
		expectedErr := errors.New("marshal failed")

		originalStatusMarshal := marshalStatusJSON
		t.Cleanup(func() { marshalStatusJSON = originalStatusMarshal })
		marshalStatusJSON = func(any, string, string) ([]byte, error) { return nil, expectedErr }
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
		})
		if _, _, err := executeRoot(t, "status", "--json"); !errors.Is(err, expectedErr) {
			t.Fatalf("expected status marshal error %v, got %v", expectedErr, err)
		}

		originalVersionMarshal := marshalVersionJSON
		t.Cleanup(func() { marshalVersionJSON = originalVersionMarshal })
		marshalVersionJSON = func(any, string, string) ([]byte, error) { return nil, expectedErr }
		if _, _, err := executeAppRoot(t, NewApp(), "version", "--json"); !errors.Is(err, expectedErr) {
			t.Fatalf("expected version marshal error %v, got %v", expectedErr, err)
		}
	})
}
