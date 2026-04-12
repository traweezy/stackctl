package config

import (
	"io"
	"testing"

	"github.com/creack/pty/v2"

	"github.com/traweezy/stackctl/internal/system"
)

func TestRunWizardUsesAccessibleHuhPathWithPTY(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")

	platform := system.CurrentPlatform()
	base := DefaultForStackOnPlatform("dev-stack", platform)
	input := wizardAnswersForState(newWizardState(base), platform, "y")

	inMaster, inTTY, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open stdin returned error: %v", err)
	}
	defer func() { _ = inMaster.Close() }()
	defer func() { _ = inTTY.Close() }()

	outMaster, outTTY, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open stdout returned error: %v", err)
	}
	defer func() { _ = outMaster.Close() }()
	defer func() { _ = outTTY.Close() }()

	go func() {
		_, _ = io.WriteString(inMaster, input)
		_ = inMaster.Close()
	}()

	cfg, err := RunWizard(inTTY, outTTY, base)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}
	if cfg.Stack.Name != base.Stack.Name || cfg.Stack.Managed != base.Stack.Managed {
		t.Fatalf("expected rich wizard path to preserve defaults, got %+v", cfg.Stack)
	}
}
