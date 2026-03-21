package cmd

import (
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestServiceContainerMapsAliases(t *testing.T) {
	cfg := configpkg.Default()

	tests := map[string]string{
		"postgres": cfg.Services.PostgresContainer,
		"pg":       cfg.Services.PostgresContainer,
		"redis":    cfg.Services.RedisContainer,
		"rd":       cfg.Services.RedisContainer,
		"pgadmin":  cfg.Services.PgAdminContainer,
	}

	for input, want := range tests {
		got, err := serviceContainer(cfg, input)
		if err != nil {
			t.Fatalf("serviceContainer(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("serviceContainer(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestServiceContainerRejectsUnknownService(t *testing.T) {
	cfg := configpkg.Default()

	if _, err := serviceContainer(cfg, "bad-service"); err == nil {
		t.Fatal("expected invalid service error")
	}
}

func TestShortID(t *testing.T) {
	if got := shortID("1234567890abcdef"); got != "1234567890ab" {
		t.Fatalf("shortID() = %q", got)
	}
}
