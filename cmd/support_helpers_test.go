package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestFilterAutoScaffoldValidationIssues(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	cfg.ApplyDerivedFields()

	issues := []configpkg.ValidationIssue{
		{Field: "stack.dir", Message: fmt.Sprintf("directory does not exist: %s", cfg.Stack.Dir)},
		{Field: "stack.compose_file", Message: fmt.Sprintf("file does not exist: %s", configpkg.ComposePath(cfg))},
		{Field: "stack.name", Message: "must not be empty"},
	}

	filtered := filterAutoScaffoldValidationIssues(cfg, issues)
	if len(filtered) != 1 || filtered[0].Field != "stack.name" {
		t.Fatalf("unexpected filtered validation issues: %+v", filtered)
	}
}

func TestScaffoldManagedStackReportsResultFlags(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")
	result := configpkg.ScaffoldResult{
		StackDir:            cfg.Stack.Dir,
		ComposePath:         configpkg.ComposePath(cfg),
		NATSConfigPath:      configpkg.NATSConfigPath(cfg),
		RedisACLPath:        configpkg.RedisACLPath(cfg),
		PgAdminServersPath:  configpkg.PgAdminServersPath(cfg),
		PGPassPath:          configpkg.PGPassPath(cfg),
		CreatedDir:          true,
		WroteCompose:        true,
		WroteNATSConfig:     true,
		WroteRedisACL:       true,
		WrotePgAdminServers: true,
		WrotePGPass:         true,
	}

	withTestDeps(t, func(d *commandDeps) {
		d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
			return result, nil
		}
	})

	cmd := &cobra.Command{Use: "support"}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := scaffoldManagedStack(cmd, cfg, true); err != nil {
		t.Fatalf("scaffoldManagedStack returned error: %v", err)
	}

	for _, fragment := range []string{
		"created managed stack directory",
		"wrote managed compose file",
		"wrote managed nats config file",
		"wrote managed redis ACL file",
		"wrote managed pgAdmin server bootstrap file",
		"wrote managed pgpass file",
	} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Fatalf("expected scaffold output to include %q:\n%s", fragment, stdout.String())
		}
	}
}

func TestScaffoldManagedStackHandlesAlreadyPresentNoopAndErrors(t *testing.T) {
	cfg := configpkg.DefaultForStack("staging")

	t.Run("already present", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{
					StackDir:       cfg.Stack.Dir,
					ComposePath:    configpkg.ComposePath(cfg),
					AlreadyPresent: true,
				}, nil
			}
		})

		cmd := &cobra.Command{Use: "support"}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)

		if err := scaffoldManagedStack(cmd, cfg, false); err != nil {
			t.Fatalf("scaffoldManagedStack returned error: %v", err)
		}
		if !strings.Contains(stdout.String(), "managed stack already exists") {
			t.Fatalf("expected already-present message, got:\n%s", stdout.String())
		}
	})

	t.Run("managed false is noop", func(t *testing.T) {
		unmanaged := cfg
		unmanaged.Stack.Managed = false

		cmd := &cobra.Command{Use: "support"}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)

		if err := scaffoldManagedStack(cmd, unmanaged, false); err != nil {
			t.Fatalf("expected nil error for unmanaged stack, got %v", err)
		}
		if strings.TrimSpace(stdout.String()) != "" {
			t.Fatalf("expected unmanaged noop to stay silent, got:\n%s", stdout.String())
		}
	})

	t.Run("propagates scaffold errors", func(t *testing.T) {
		expectedErr := errors.New("scaffold failed")
		withTestDeps(t, func(d *commandDeps) {
			d.scaffoldManagedStack = func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{}, expectedErr
			}
		})

		cmd := &cobra.Command{Use: "support"}
		if err := scaffoldManagedStack(cmd, cfg, true); !errors.Is(err, expectedErr) {
			t.Fatalf("expected scaffold error %v, got %v", expectedErr, err)
		}
	})
}
