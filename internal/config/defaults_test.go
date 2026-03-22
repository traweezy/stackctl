package config

import "testing"

func TestDefaultConfigHasDerivedURLs(t *testing.T) {
	cfg := Default()

	if cfg.URLs.Cockpit != "https://localhost:9090" {
		t.Fatalf("unexpected cockpit URL: %s", cfg.URLs.Cockpit)
	}

	if cfg.URLs.PgAdmin != "http://localhost:8081" {
		t.Fatalf("unexpected pgadmin URL: %s", cfg.URLs.PgAdmin)
	}
	if cfg.Connection.RedisPassword != "" {
		t.Fatalf("expected redis password to default to empty, got %q", cfg.Connection.RedisPassword)
	}
	if cfg.Connection.PgAdminEmail != "admin@example.com" {
		t.Fatalf("unexpected pgadmin email: %s", cfg.Connection.PgAdminEmail)
	}
	if cfg.Connection.PgAdminPassword != "admin" {
		t.Fatalf("unexpected pgadmin password: %s", cfg.Connection.PgAdminPassword)
	}
}
