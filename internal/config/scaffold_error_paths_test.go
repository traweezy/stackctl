package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func selectiveParseScaffoldError(t *testing.T, targetName string, expectedErr error) {
	t.Helper()

	originalParse := parseScaffoldTemplate
	t.Cleanup(func() { parseScaffoldTemplate = originalParse })
	parseScaffoldTemplate = func(name, text string) (*template.Template, error) {
		if name == targetName {
			return nil, expectedErr
		}
		return originalParse(name, text)
	}
}

func scaffoldConfigForTemplate(targetName string) Config {
	cfg := Default()
	switch targetName {
	case "dev-stack-nats":
		cfg.Setup.IncludeNATS = true
	case "dev-stack-redis-acl":
		cfg.Connection.RedisACLUsername = "stack-user"
		cfg.Connection.RedisACLPassword = "stack-pass"
	case "dev-stack-pgadmin-servers", "dev-stack-pgpass":
		cfg.Setup.IncludePgAdmin = true
		cfg.Services.PgAdmin.BootstrapPostgresServer = true
	}
	cfg.ApplyDerivedFields()
	return cfg
}

func TestExecuteScaffoldTemplate(t *testing.T) {
	t.Run("writes template output", func(t *testing.T) {
		tmpl, err := template.New("test").Parse("hello {{.}}")
		if err != nil {
			t.Fatalf("Parse returned error: %v", err)
		}

		data, err := executeScaffoldTemplate(tmpl, "world")
		if err != nil {
			t.Fatalf("executeScaffoldTemplate returned error: %v", err)
		}
		if got := string(data); got != "hello world" {
			t.Fatalf("unexpected rendered template output %q", got)
		}
	})

	t.Run("surfaces template execution errors", func(t *testing.T) {
		tmpl, err := template.New("test").Parse("hello {{.Name}}")
		if err != nil {
			t.Fatalf("Parse returned error: %v", err)
		}

		if _, err := executeScaffoldTemplate(tmpl, struct{}{}); err == nil {
			t.Fatal("expected executeScaffoldTemplate to surface template execution errors")
		}
	})
}

func TestManagedStackScaffoldErrorPaths(t *testing.T) {
	t.Run("ManagedStackNeedsScaffold returns false when managed scaffolding is disabled", func(t *testing.T) {
		cfg := Default()
		cfg.Setup.ScaffoldDefaultStack = false
		cfg.ApplyDerivedFields()

		needsScaffold, err := ManagedStackNeedsScaffold(cfg)
		if err != nil || needsScaffold {
			t.Fatalf("expected disabled managed scaffolding to return false, got needsScaffold=%v err=%v", needsScaffold, err)
		}
	})

	t.Run("ManagedStackNeedsScaffold surfaces injected render failures", func(t *testing.T) {
		testCases := []struct {
			name       string
			targetName string
		}{
			{name: "compose", targetName: "dev-stack-compose"},
			{name: "nats", targetName: "dev-stack-nats"},
			{name: "redis acl", targetName: "dev-stack-redis-acl"},
			{name: "pgadmin servers", targetName: "dev-stack-pgadmin-servers"},
			{name: "pgpass", targetName: "dev-stack-pgpass"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Setenv("XDG_DATA_HOME", t.TempDir())
				cfg := scaffoldConfigForTemplate(tc.targetName)
				if tc.targetName != "dev-stack-compose" {
					if _, err := ScaffoldManagedStack(cfg, false); err != nil {
						t.Fatalf("ScaffoldManagedStack returned error: %v", err)
					}
				}

				expectedErr := errors.New("render failed")
				selectiveParseScaffoldError(t, tc.targetName, expectedErr)

				if _, err := ManagedStackNeedsScaffold(cfg); !errors.Is(err, expectedErr) {
					t.Fatalf("expected ManagedStackNeedsScaffold to surface %v, got %v", expectedErr, err)
				}
			})
		}
	})

	t.Run("ScaffoldManagedStack surfaces injected render failures", func(t *testing.T) {
		testCases := []struct {
			name       string
			targetName string
		}{
			{name: "compose", targetName: "dev-stack-compose"},
			{name: "nats", targetName: "dev-stack-nats"},
			{name: "redis acl", targetName: "dev-stack-redis-acl"},
			{name: "pgadmin servers", targetName: "dev-stack-pgadmin-servers"},
			{name: "pgpass", targetName: "dev-stack-pgpass"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Setenv("XDG_DATA_HOME", t.TempDir())
				cfg := scaffoldConfigForTemplate(tc.targetName)
				expectedErr := errors.New("render failed")
				selectiveParseScaffoldError(t, tc.targetName, expectedErr)

				if _, err := ScaffoldManagedStack(cfg, false); !errors.Is(err, expectedErr) {
					t.Fatalf("expected ScaffoldManagedStack to surface %v, got %v", expectedErr, err)
				}
			})
		}
	})

	t.Run("scaffoldFileNeedsWrite surfaces open and read failures", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "compose.yaml")
		if err := os.WriteFile(target, []byte("services: {}\n"), 0o600); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		originalOpenRoot := openScaffoldRoot
		originalReadFile := readScaffoldFile
		t.Cleanup(func() {
			openScaffoldRoot = originalOpenRoot
			readScaffoldFile = originalReadFile
		})

		openScaffoldRoot = func(string) (*os.Root, error) {
			return nil, errors.New("open root failed")
		}
		if _, err := scaffoldFileNeedsWrite(target, []byte("services: {}\n")); err == nil || !strings.Contains(err.Error(), "open root failed") {
			t.Fatalf("expected open root failure, got %v", err)
		}

		openScaffoldRoot = originalOpenRoot
		readScaffoldFile = func(*os.Root, string) ([]byte, error) {
			return nil, errors.New("read failed")
		}
		if _, err := scaffoldFileNeedsWrite(target, []byte("services: {}\n")); err == nil || !strings.Contains(err.Error(), "read failed") {
			t.Fatalf("expected read failure, got %v", err)
		}
	})
}
