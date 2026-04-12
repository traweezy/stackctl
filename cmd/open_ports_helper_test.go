package cmd

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

func TestOpenHandlesDisabledTargetsAndAllSelection(t *testing.T) {
	t.Run("cockpit disabled", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) {
				cfg := configpkg.Default()
				cfg.Setup.IncludeCockpit = false
				cfg.ApplyDerivedFields()
				return cfg, nil
			}
		})

		_, _, err := executeRoot(t, "open", "cockpit")
		if err == nil || !strings.Contains(err.Error(), "cockpit is disabled in config") {
			t.Fatalf("unexpected cockpit error: %v", err)
		}
	})

	t.Run("meilisearch disabled", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) {
				cfg := configpkg.Default()
				cfg.Setup.IncludeMeilisearch = false
				cfg.ApplyDerivedFields()
				return cfg, nil
			}
		})

		_, _, err := executeRoot(t, "open", "meilisearch")
		if err == nil || !strings.Contains(err.Error(), "meilisearch is disabled in config") {
			t.Fatalf("unexpected meilisearch error: %v", err)
		}
	})

	t.Run("open all respects enabled target order", func(t *testing.T) {
		var opened []string

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) {
				cfg := configpkg.Default()
				cfg.Setup.IncludeMeilisearch = true
				cfg.Setup.IncludePgAdmin = true
				cfg.Setup.IncludeCockpit = true
				cfg.ApplyDerivedFields()
				return cfg, nil
			}
			d.openURL = func(_ context.Context, _ system.Runner, target string) error {
				opened = append(opened, target)
				return nil
			}
		})

		_, _, err := executeRoot(t, "open", "all")
		if err != nil {
			t.Fatalf("open all returned error: %v", err)
		}
		expected := []string{
			"https://localhost:9090",
			"http://localhost:7700",
			"http://localhost:8081",
		}
		if !reflect.DeepEqual(opened, expected) {
			t.Fatalf("opened targets = %v, want %v", opened, expected)
		}
	})
}

func TestWritePortMappingsAndFormatPortMappings(t *testing.T) {
	mappings := []portMapping{
		{Service: "postgres", DisplayName: "Postgres", Host: "devbox", ExternalPort: 15432, InternalPort: 5432},
		{Service: "redis", DisplayName: "Redis", Host: "devbox", ExternalPort: 16379, InternalPort: 6379},
	}

	var out bytes.Buffer
	if err := writePortMappings(&out, mappings); err != nil {
		t.Fatalf("writePortMappings returned error: %v", err)
	}

	rendered := out.String()
	for _, fragment := range []string{
		"SERVICE",
		"Postgres",
		"Redis",
		"15432 -> 5432",
		"16379 -> 6379",
	} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("expected port table to contain %q:\n%s", fragment, rendered)
		}
	}
	if got := formatPortMappings(mappings); got != strings.TrimSpace(rendered) {
		t.Fatalf("unexpected formatted mappings %q want %q", got, strings.TrimSpace(rendered))
	}
}
