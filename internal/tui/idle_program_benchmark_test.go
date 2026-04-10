package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

const idleProgramBenchmarkDurationEnv = "STACKCTL_IDLE_BENCH_DURATION"

type idleProgramModel struct {
	Model
}

func (m idleProgramModel) Init() tea.Cmd {
	return nil
}

func (m idleProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.Model.Update(msg)
	return idleProgramModel{Model: updated.(Model)}, cmd
}

func BenchmarkIdleProgramDefaultFPS(b *testing.B) {
	benchmarkIdleProgram(b, 0)
}

func BenchmarkIdleProgramFPS30(b *testing.B) {
	benchmarkIdleProgram(b, 30)
}

func benchmarkIdleProgram(b *testing.B, fps int) {
	duration := idleProgramBenchmarkDuration()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		model := benchmarkModel(servicesSection)
		model.autoRefresh = false
		model.mouseEnabled = false
		model.altScreen = false

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		options := []tea.ProgramOption{
			tea.WithContext(ctx),
			tea.WithInput(nil),
			tea.WithOutput(io.Discard),
			tea.WithWindowSize(model.width, model.height),
			tea.WithoutSignals(),
		}
		if fps > 0 {
			options = append(options, tea.WithFPS(fps))
		}

		program := tea.NewProgram(idleProgramModel{Model: model}, options...)
		if _, err := program.Run(); err != nil && !errors.Is(err, tea.ErrProgramKilled) {
			cancel()
			b.Fatalf("run idle program: %v", err)
		}
		cancel()
	}
}

func idleProgramBenchmarkDuration() time.Duration {
	if raw := os.Getenv(idleProgramBenchmarkDurationEnv); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 5 * time.Second
}
