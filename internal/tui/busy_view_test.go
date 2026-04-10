package tui

import (
	"testing"
	"time"

	bubblespinner "charm.land/bubbles/v2/spinner"
	bubblesstopwatch "charm.land/bubbles/v2/stopwatch"
)

func TestBusySpinnerTickPreservesViewportContentVersionWhenLayoutIsStable(t *testing.T) {
	model := benchmarkModel(servicesSection)
	model.loading = true
	_ = model.beginBusy(0)
	model.syncLayout()

	before := model.contentVersion
	updated, cmd := model.Update(bubblespinner.TickMsg{})
	current := updated.(Model)

	if cmd == nil {
		t.Fatal("expected busy spinner tick to schedule follow-up work")
	}
	if current.contentVersion != before {
		t.Fatalf("expected busy spinner tick to preserve content version, before=%d after=%d", before, current.contentVersion)
	}
}

func TestBusyStopwatchTickPreservesViewportContentVersionWhenLayoutIsStable(t *testing.T) {
	model := benchmarkModel(servicesSection)
	model.runningAction = &runningAction{Action: ActionSpec{ID: ActionStart, Label: "Start stack"}}
	_ = model.beginBusy(30 * time.Second)
	model.syncLayout()

	before := model.contentVersion
	updated, _ := model.Update(bubblesstopwatch.TickMsg{})
	current := updated.(Model)

	if !current.isBusy() {
		t.Fatal("expected busy stopwatch tick to preserve the busy state")
	}
	if current.contentVersion != before {
		t.Fatalf("expected busy stopwatch tick to preserve content version, before=%d after=%d", before, current.contentVersion)
	}
}

func TestBusySpinnerTickRefreshesViewportContentWhenFrameResizes(t *testing.T) {
	model := benchmarkModel(servicesSection)
	model.loading = true
	_ = model.beginBusy(0)
	model.syncLayout()
	model.viewport.SetWidth(model.viewport.Width() - 1)
	model.viewport.SetHeight(model.viewport.Height() - 1)

	before := model.contentVersion
	updated, _ := model.Update(bubblespinner.TickMsg{})
	current := updated.(Model)

	if current.contentVersion != before+1 {
		t.Fatalf("expected busy spinner tick to refresh content after a resize, before=%d after=%d", before, current.contentVersion)
	}
	if current.viewport.Width() <= 1 || current.viewport.Height() <= 1 {
		t.Fatalf("expected busy spinner tick to restore viewport dimensions, got %dx%d", current.viewport.Width(), current.viewport.Height())
	}
}
