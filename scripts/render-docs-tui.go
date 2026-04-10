//go:build docs_capture

package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	stacktui "github.com/traweezy/stackctl/internal/tui"
)

const docsCaptureDurationEnv = "STACKCTL_DOCS_TUI_CAPTURE_DURATION"

func main() {
	duration := docsCaptureDuration()
	model := stacktui.NewFullModel(func() (stacktui.Snapshot, error) {
		return docsCaptureSnapshot(), nil
	}, nil, nil, nil).
		WithVersion("1.0.0").
		WithMouse(false).
		WithAltScreen(false).
		WithHelpExpanded(true)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 140, Height: 42})
	current := updated.(stacktui.Model)

	if cmd := current.Init(); cmd != nil {
		current = applyCaptureMsg(current, cmd())
	}

	for range 3 {
		updated, _ = current.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
		current = updated.(stacktui.Model)
	}

	view := current.View()
	_, _ = fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H\x1b[?25l", view.Content)
	defer fmt.Fprint(os.Stdout, "\x1b[?25h")

	time.Sleep(duration)
}

func docsCaptureDuration() time.Duration {
	if raw := os.Getenv(docsCaptureDurationEnv); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 10 * time.Second
}

func docsCaptureSnapshot() stacktui.Snapshot {
	return stacktui.Snapshot{
		StackName:         "dev-stack",
		StackDir:          "/tmp/stackctl/dev-stack",
		ComposePath:       "/tmp/stackctl/dev-stack/compose.yaml",
		Managed:           true,
		WaitForServices:   true,
		StartupTimeoutSec: 45,
		LoadedAt:          time.Unix(1_714_000_000, 0),
		Services: []stacktui.Service{
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
				Name:              "redis",
				DisplayName:       "Redis",
				Status:            "running",
				ContainerName:     "redis",
				Image:             "redis:7",
				DataVolume:        "redis_data",
				Host:              "127.0.0.1",
				ExternalPort:      6379,
				InternalPort:      6379,
				PortListening:     true,
				AppendOnly:        boolPtr(true),
				SavePolicy:        "60 1000",
				MaxMemoryPolicy:   "allkeys-lru",
				VolumeSizeLimitMB: 1024,
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
			{
				Name:              "meilisearch",
				DisplayName:       "Meilisearch",
				Status:            "running",
				ContainerName:     "meilisearch",
				Image:             "getmeili/meilisearch:v1.14",
				DataVolume:        "meilisearch_data",
				Host:              "127.0.0.1",
				ExternalPort:      7700,
				InternalPort:      7700,
				PortListening:     true,
				URL:               "http://127.0.0.1:7700",
				VolumeSizeLimitMB: 1024,
			},
		},
		Health: []stacktui.HealthLine{
			{Status: "OK", Message: "Postgres is accepting connections on 127.0.0.1:5432"},
			{Status: "OK", Message: "Redis is accepting connections on 127.0.0.1:6379"},
			{Status: "OK", Message: "pgAdmin is reachable on http://127.0.0.1:5050"},
			{Status: "OK", Message: "Meilisearch is reachable on http://127.0.0.1:7700"},
		},
		Stacks: []stacktui.StackProfile{
			{
				Name:       "dev-stack",
				ConfigPath: "/tmp/stackctl/config.yaml",
				Current:    true,
				Configured: true,
				State:      "running",
				Mode:       "managed",
				Services:   "postgres, redis, pgadmin, meilisearch",
			},
			{
				Name:       "staging",
				ConfigPath: "/tmp/stackctl/stacks/staging.yaml",
				Configured: true,
				State:      "stopped",
				Mode:       "external",
				Services:   "postgres, redis",
			},
		},
		Connections: []stacktui.Connection{
			{Name: "Postgres", Value: "postgres://postgres:postgres@127.0.0.1:5432/app?sslmode=disable"},
			{Name: "Redis", Value: "redis://127.0.0.1:6379/0"},
			{Name: "pgAdmin", Value: "http://127.0.0.1:5050"},
			{Name: "Meilisearch", Value: "http://127.0.0.1:7700"},
		},
		ConnectText:   "export DATABASE_URL=postgres://postgres:postgres@127.0.0.1:5432/app?sslmode=disable",
		EnvExportText: "export PGHOST=127.0.0.1\nexport REDIS_URL=redis://127.0.0.1:6379/0",
		PortsText:     "postgres 5432\nredis 6379\npgadmin 5050\nmeilisearch 7700",
	}
}

func applyCaptureMsg(model stacktui.Model, msg tea.Msg) stacktui.Model {
	switch typed := msg.(type) {
	case tea.BatchMsg:
		current := model
		for _, cmd := range typed {
			if cmd == nil {
				continue
			}
			current = applyCaptureMsg(current, cmd())
		}
		return current
	default:
		updated, _ := model.Update(typed)
		return updated.(stacktui.Model)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
