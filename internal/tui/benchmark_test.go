package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

var benchmarkViewSink string
var benchmarkCountSink int

func BenchmarkViewOverview(b *testing.B) {
	model := benchmarkModel(overviewSection)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkViewSink = model.View().Content
	}
}

func BenchmarkViewServices(b *testing.B) {
	model := benchmarkModel(servicesSection)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkViewSink = model.View().Content
	}
}

func BenchmarkPaletteApplyFilter(b *testing.B) {
	items := make([]paletteAction, 0, 500)
	for i := 0; i < 500; i++ {
		service := fmt.Sprintf("service-%03d", i)
		items = append(items, paletteAction{
			Kind:       paletteActionJumpService,
			Title:      "Go to " + service,
			Subtitle:   fmt.Sprintf("running • %s", service),
			Search:     strings.ToLower(service + " go running service jump"),
			ServiceKey: service,
		})
	}
	items = append(items,
		paletteAction{Kind: paletteActionJumpService, Title: "Go to postgres", Subtitle: "running", Search: "postgres go running service jump", ServiceKey: "postgres"},
		paletteAction{Kind: paletteActionJumpService, Title: "Go to pgadmin", Subtitle: "running", Search: "pgadmin go running service jump", ServiceKey: "pgadmin"},
	)

	state := newPaletteState(paletteModeJump, "Jump", "Search", items)
	state.input.SetValue("pg")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.applyFilter()
		benchmarkCountSink = len(state.filtered)
	}
}

func benchmarkModel(active section) Model {
	model := NewModel(func() (Snapshot, error) {
		return Snapshot{}, nil
	}).WithVersion("1.0.0")
	model.width = 140
	model.height = 42
	model.loading = false
	model.autoRefresh = true
	model.active = active
	model.snapshot = benchmarkSnapshot()
	model.selectedService = "postgres"
	model.selectedHealth = "postgres"
	model.selectedStack = "dev-stack"
	model.syncLayout()
	return model
}

func benchmarkSnapshot() Snapshot {
	return Snapshot{
		StackName:         "dev-stack",
		StackDir:          "/tmp/stackctl/dev-stack",
		ComposePath:       "/tmp/stackctl/dev-stack/compose.yaml",
		Managed:           true,
		WaitForServices:   true,
		StartupTimeoutSec: 45,
		LoadedAt:          time.Unix(1_714_000_000, 0),
		Services: []Service{
			{
				Name:          "postgres",
				DisplayName:   "Postgres",
				Status:        "running",
				ContainerName: "postgres",
				Image:         "postgres:17",
				DataVolume:    "postgres_data",
				Host:          "127.0.0.1",
				ExternalPort:  5432,
				InternalPort:  5432,
				PortListening: true,
				Database:      "app",
				MaintenanceDB: "postgres",
				Username:      "postgres",
				Password:      "postgres",
				DSN:           "postgres://postgres:postgres@127.0.0.1:5432/app?sslmode=disable",
			},
			{
				Name:            "redis",
				DisplayName:     "Redis",
				Status:          "running",
				ContainerName:   "redis",
				Image:           "redis:7",
				DataVolume:      "redis_data",
				Host:            "127.0.0.1",
				ExternalPort:    6379,
				InternalPort:    6379,
				PortListening:   true,
				AppendOnly:      boolPtr(true),
				SavePolicy:      "60 1000",
				MaxMemoryPolicy: "allkeys-lru",
			},
			{
				Name:              "pgadmin",
				DisplayName:       "pgAdmin",
				Status:            "running",
				ContainerName:     "pgadmin",
				Image:             "dpage/pgadmin4:latest",
				DataVolume:        "pgadmin_data",
				Host:              "127.0.0.1",
				ExternalPort:      5050,
				InternalPort:      80,
				PortListening:     true,
				Email:             "admin@example.test",
				URL:               "http://127.0.0.1:5050",
				VolumeSizeLimitMB: 2048,
			},
		},
		Health: []HealthLine{
			{Status: "OK", Message: "Postgres is accepting connections on 127.0.0.1:5432"},
			{Status: "OK", Message: "Redis is accepting connections on 127.0.0.1:6379"},
			{Status: "OK", Message: "pgAdmin is reachable on http://127.0.0.1:5050"},
		},
		Stacks: []StackProfile{
			{
				Name:       "dev-stack",
				ConfigPath: "/tmp/stackctl/config.yaml",
				Current:    true,
				Configured: true,
				State:      "running",
				Mode:       "managed",
				Services:   "postgres, redis, pgadmin",
			},
			{
				Name:       "staging",
				ConfigPath: "/tmp/stackctl/stacks/staging.yaml",
				Current:    false,
				Configured: true,
				State:      "stopped",
				Mode:       "external",
				Services:   "postgres, redis",
			},
		},
		Connections: []Connection{
			{Name: "Postgres", Value: "postgres://postgres:postgres@127.0.0.1:5432/app?sslmode=disable"},
			{Name: "Redis", Value: "redis://127.0.0.1:6379/0"},
			{Name: "pgAdmin", Value: "http://127.0.0.1:5050"},
		},
		ConnectText:   "export DATABASE_URL=postgres://postgres:postgres@127.0.0.1:5432/app?sslmode=disable",
		EnvExportText: "export PGHOST=127.0.0.1",
		PortsText:     "postgres 5432\nredis 6379\npgadmin 5050",
	}
}

func boolPtr(value bool) *bool {
	return &value
}
