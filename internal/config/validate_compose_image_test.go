package config

import (
	"errors"
	"os"
	"strings"
	"testing"

	composecli "github.com/compose-spec/compose-go/v2/cli"
)

func TestValidateRejectsInvalidContainerImageReferences(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if err := validateWithDir(cfg.Stack.Dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ComposePath(cfg), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg.Services.Postgres.Image = "%%%not-a-reference%%%"
	cfg.Services.Redis.Image = "%%%still-bad%%%"
	cfg.Services.NATS.Image = "%%%bad-nats%%%"
	cfg.Setup.IncludeSeaweedFS = true
	cfg.Services.SeaweedFS.Image = "%%%bad-seaweedfs%%%"
	cfg.Setup.IncludeMeilisearch = true
	cfg.Services.Meilisearch.Image = "%%%bad-meilisearch%%%"
	cfg.Services.PgAdmin.Image = "%%%bad-pgadmin%%%"

	issues := Validate(cfg)
	fields := map[string]string{}
	for _, issue := range issues {
		fields[issue.Field] = issue.Message
	}

	for _, field := range []string{
		"services.postgres.image",
		"services.redis.image",
		"services.nats.image",
		"services.seaweedfs.image",
		"services.meilisearch.image",
		"services.pgadmin.image",
	} {
		message, ok := fields[field]
		if !ok {
			t.Fatalf("expected validation issue for %s, got %v", field, issues)
		}
		if !strings.Contains(message, "valid container image reference") {
			t.Fatalf("expected image-reference validation message for %s, got %q", field, message)
		}
	}
}

func TestValidateRejectsInvalidManagedComposeFile(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if err := validateWithDir(cfg.Stack.Dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ComposePath(cfg), []byte("services:\n  postgres: [broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	issues := Validate(cfg)
	for _, issue := range issues {
		if issue.Field == "stack.compose_file" && strings.Contains(issue.Message, "managed compose file is invalid") {
			return
		}
	}

	t.Fatalf("expected invalid managed compose issue, got %v", issues)
}

func TestScaffoldManagedStackWritesComposeFileThatLoads(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	if err := validateManagedComposeFile(cfg); err != nil {
		t.Fatalf("expected scaffolded compose file to parse, got %v", err)
	}
}

func TestValidateManagedComposeFileSurfacesProjectOptionFailures(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	expectedErr := errors.New("project options broke")

	previous := newComposeProjectOptions
	newComposeProjectOptions = func(configs []string, opts ...composecli.ProjectOptionsFn) (*composecli.ProjectOptions, error) {
		return nil, expectedErr
	}
	defer func() { newComposeProjectOptions = previous }()

	err := validateManagedComposeFile(cfg)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
