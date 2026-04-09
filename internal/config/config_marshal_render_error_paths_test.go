package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestSaveAndMarshalSurfaceInjectedMarshalErrors(t *testing.T) {
	originalMarshal := marshalConfigYAML
	t.Cleanup(func() { marshalConfigYAML = originalMarshal })

	expectedErr := errors.New("marshal failed")
	marshalConfigYAML = func(any) ([]byte, error) {
		return nil, expectedErr
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, Default()); !errors.Is(err, expectedErr) {
		t.Fatalf("expected Save to surface %v, got %v", expectedErr, err)
	}

	if _, err := Marshal(Default()); !errors.Is(err, expectedErr) {
		t.Fatalf("expected Marshal to surface %v, got %v", expectedErr, err)
	}
}

func TestRenderManagedTemplatesSurfaceParseAndExecuteErrors(t *testing.T) {
	cfg := Default()
	cfg.Stack.Managed = true
	cfg.Setup.ScaffoldDefaultStack = true
	cfg.ApplyDerivedFields()

	testCases := []struct {
		name     string
		render   func(Config) ([]byte, error)
		parseMsg string
		execMsg  string
	}{
		{name: "compose", render: renderManagedCompose, parseMsg: "parse embedded compose template", execMsg: "render managed compose template"},
		{name: "nats", render: renderManagedNATSConfig, parseMsg: "parse embedded nats template", execMsg: "render managed nats template"},
		{name: "redis acl", render: renderManagedRedisACL, parseMsg: "parse embedded redis ACL template", execMsg: "render managed redis ACL template"},
		{name: "pgadmin servers", render: renderManagedPgAdminServers, parseMsg: "parse embedded pgAdmin server template", execMsg: "render managed pgAdmin server template"},
		{name: "pgpass", render: renderManagedPGPass, parseMsg: "parse embedded pgpass template", execMsg: "render managed pgpass template"},
	}

	for _, tc := range testCases {
		t.Run(tc.name+" parse error", func(t *testing.T) {
			originalParse := parseScaffoldTemplate
			originalExecute := executeScaffoldTemplate
			t.Cleanup(func() {
				parseScaffoldTemplate = originalParse
				executeScaffoldTemplate = originalExecute
			})

			parseScaffoldTemplate = func(string, string) (*template.Template, error) {
				return nil, errors.New("parse failed")
			}

			if _, err := tc.render(cfg); err == nil || !strings.Contains(err.Error(), tc.parseMsg) {
				t.Fatalf("expected %q error, got %v", tc.parseMsg, err)
			}
		})

		t.Run(tc.name+" execute error", func(t *testing.T) {
			originalParse := parseScaffoldTemplate
			originalExecute := executeScaffoldTemplate
			t.Cleanup(func() {
				parseScaffoldTemplate = originalParse
				executeScaffoldTemplate = originalExecute
			})

			executeScaffoldTemplate = func(*template.Template, any) ([]byte, error) {
				return nil, errors.New("execute failed")
			}

			if _, err := tc.render(cfg); err == nil || !strings.Contains(err.Error(), tc.execMsg) {
				t.Fatalf("expected %q error, got %v", tc.execMsg, err)
			}
		})
	}
}
