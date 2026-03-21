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
}
