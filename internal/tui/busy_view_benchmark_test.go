package tui

import (
	"testing"
	"time"

	bubblespinner "charm.land/bubbles/v2/spinner"
	bubblesstopwatch "charm.land/bubbles/v2/stopwatch"
)

func BenchmarkBusySpinnerTickServices(b *testing.B) {
	model := benchmarkModel(servicesSection)
	model.loading = true
	_ = model.beginBusy(0)
	model.syncLayout()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updated, _ := model.Update(bubblespinner.TickMsg{})
		model = updated.(Model)
		benchmarkViewSink = model.View().Content
	}
}

func BenchmarkBusyStopwatchTickServices(b *testing.B) {
	model := benchmarkModel(servicesSection)
	model.runningAction = &runningAction{Action: ActionSpec{ID: ActionStart, Label: "Start stack"}}
	_ = model.beginBusy(30 * time.Second)
	model.syncLayout()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updated, _ := model.Update(bubblesstopwatch.TickMsg{})
		model = updated.(Model)
		benchmarkViewSink = model.View().Content
	}
}
