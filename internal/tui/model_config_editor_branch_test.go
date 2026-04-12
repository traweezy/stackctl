package tui

import (
	"strings"
	"testing"
	"time"

	bubblesstopwatch "charm.land/bubbles/v2/stopwatch"
	bubblestimer "charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func TestTUIModelUpdateCoverageBatchSix(t *testing.T) {
	cfg := configpkg.Default()
	postgres := Service{
		Name:          "postgres",
		DisplayName:   "Postgres",
		Status:        "running",
		ContainerName: cfg.Services.PostgresContainer,
		ExternalPort:  cfg.Ports.Postgres,
	}
	snapshot := tuiTestSnapshot(cfg, []Service{postgres})

	t.Run("idle timer and stopwatch ticks return no follow-up command", func(t *testing.T) {
		current := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)
		current.busyStopwatch = bubblesstopwatch.New(bubblesstopwatch.WithInterval(time.Second))
		current.busyTimer = bubblestimer.New(time.Second, bubblestimer.WithInterval(time.Second))
		current.busyBudget = time.Second

		updated, cmd := current.Update(bubblesstopwatch.TickMsg{})
		current = updated.(Model)
		if cmd != nil {
			t.Fatalf("expected idle stopwatch tick to return nil, got %v", cmd)
		}

		_, cmd = current.Update(bubblestimer.TickMsg{})
		if cmd != nil {
			t.Fatalf("expected idle timer tick to return nil, got %v", cmd)
		}
	})

	t.Run("config operation without reload just clears busy state", func(t *testing.T) {
		current := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)
		current.runningConfigOp = &configOperation{Message: "Saving config"}
		current.beginBusy(0)

		updated, cmd := current.Update(configOperationMsg{
			Status:  output.StatusOK,
			Message: "saved config",
		})
		current = updated.(Model)
		if current.runningConfigOp != nil {
			t.Fatalf("expected config operation to clear, got %+v", current.runningConfigOp)
		}
		if current.loading {
			t.Fatal("expected non-reload config operation to stay out of loading mode")
		}
		if cmd == nil {
			t.Fatal("expected config operation completion to return a banner clear command")
		}
	})

	t.Run("key handling covers blocked and direct-return branches", func(t *testing.T) {
		actionRunner := func(ActionID) (ActionReport, error) {
			return ActionReport{Status: output.StatusOK, Message: "ok"}, nil
		}
		current := loadSnapshotModel(t, NewFullModel(
			func() (Snapshot, error) { return snapshot, nil },
			nil,
			actionRunner,
			nil,
		), snapshot)

		current.active = configSection
		current.selectedService = serviceKey(postgres)

		for _, tc := range []struct {
			name string
			msg  tea.KeyPressMsg
		}{
			{name: "exec shell blocked", msg: tea.KeyPressMsg{Code: 'e', Text: "e"}},
			{name: "db shell blocked", msg: tea.KeyPressMsg{Code: 'd', Text: "d"}},
			{name: "pin service blocked", msg: tea.KeyPressMsg{Code: 'p', Text: "p"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				updated, cmd := current.Update(tc.msg)
				if cmd != nil {
					t.Fatalf("expected blocked key path to return nil, got %v", cmd)
				}
				if updated.(Model).active != configSection {
					t.Fatalf("expected blocked key path to keep config section active, got %+v", updated)
				}
			})
		}

		current.active = overviewSection
		updated, cmd := current.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
		if cmd == nil {
			t.Fatal("expected quit key to return tea.Quit")
		}
		if _, ok := updated.(Model); !ok {
			t.Fatalf("expected quit branch to keep returning a Model, got %T", updated)
		}

		current.layout = compactLayout
		updated, cmd = current.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
		current = updated.(Model)
		if cmd != nil || current.layout != expandedLayout {
			t.Fatalf("expected toggle-layout branch to switch back to expanded layout, model=%+v cmd=%v", current, cmd)
		}

		current.active = overviewSection
		_, cmd = current.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
		if cmd != nil {
			t.Fatalf("expected prev-item on a section without selections to return nil, got %v", cmd)
		}

		current.active = servicesSection
		current.autoRefresh = true
		current.selectedService = serviceKey(postgres)
		updated, cmd = current.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
		current = updated.(Model)
		if cmd == nil {
			t.Fatal("expected prev-item with auto-refresh enabled to return a batch command")
		}
		if current.selectedService == "" {
			t.Fatalf("expected selected service to remain set, got %+v", current)
		}
	})

	t.Run("action key returns nil when no runner is configured", func(t *testing.T) {
		current := loadSnapshotModel(t, NewModel(func() (Snapshot, error) { return snapshot, nil }), snapshot)
		updated, cmd := current.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
		if cmd != nil {
			t.Fatalf("expected action key to be ignored without a runner, got %v", cmd)
		}
		if _, ok := updated.(Model); !ok {
			t.Fatalf("expected ignored action update to keep the model type, got %T", updated)
		}
	})
}

func TestConfigEditorCoverageBatchSix(t *testing.T) {
	t.Run("applyFollowUpMessage covers the default empty branch", func(t *testing.T) {
		if got := applyFollowUpMessage(configpkg.Default(), configpkg.Default(), configApplyPlan{}); got != "" {
			t.Fatalf("expected empty follow-up for a no-op plan, got %q", got)
		}
	})

	t.Run("saveFollowUpMessage covers local-only and review-required branches", func(t *testing.T) {
		localPrevious := configpkg.Default()
		localNext := localPrevious
		localNext.Connection.Host = "db.internal"
		if got := saveFollowUpMessage(localPrevious, localNext, 1); got != "running services were not changed" {
			t.Fatalf("expected local-only save follow-up, got %q", got)
		}

		reviewPrevious := configpkg.Default()
		reviewPrevious.Setup.ScaffoldDefaultStack = false
		reviewPrevious.ApplyDerivedFields()
		reviewNext := reviewPrevious
		reviewNext.Services.Postgres.MaxConnections++
		if got := saveFollowUpMessage(reviewPrevious, reviewNext, 0); !strings.Contains(got, "update the managed compose file yourself") {
			t.Fatalf("expected managed scaffold-disabled follow-up, got %q", got)
		}
	})

	t.Run("selectedFieldEffect returns follow-up text for unknown fields", func(t *testing.T) {
		cfg := configpkg.Default()
		got := selectedFieldEffect(configFieldSpec{Key: "custom"}, cfg)
		if !strings.Contains(got, "refreshes compose automatically") {
			t.Fatalf("expected follow-up-only effect message, got %q", got)
		}
	})
}
