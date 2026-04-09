package config

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	huh "charm.land/huh/v2"

	"github.com/traweezy/stackctl/internal/system"
)

func TestRunHuhWizardWithPlatformErrorPaths(t *testing.T) {
	platform := system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	}
	base := DefaultForStackOnPlatform("dev-stack", platform)

	t.Run("form run error", func(t *testing.T) {
		originalRunForm := runHuhForm
		originalRunReview := runHuhReviewForm
		originalReviewStep := runWizardReviewStep
		originalToConfig := wizardStateToConfigForPlatform
		t.Cleanup(func() {
			runHuhForm = originalRunForm
			runHuhReviewForm = originalRunReview
			runWizardReviewStep = originalReviewStep
			wizardStateToConfigForPlatform = originalToConfig
		})

		expectedErr := errors.New("form failed")
		runHuhForm = func(*huh.Form) error { return expectedErr }

		if _, err := runHuhWizardWithPlatform(strings.NewReader(""), io.Discard, base, platform); !errors.Is(err, expectedErr) {
			t.Fatalf("expected form error %v, got %v", expectedErr, err)
		}
	})

	t.Run("review run error", func(t *testing.T) {
		originalRunForm := runHuhForm
		originalRunReview := runHuhReviewForm
		originalReviewStep := runWizardReviewStep
		originalToConfig := wizardStateToConfigForPlatform
		t.Cleanup(func() {
			runHuhForm = originalRunForm
			runHuhReviewForm = originalRunReview
			runWizardReviewStep = originalReviewStep
			wizardStateToConfigForPlatform = originalToConfig
		})

		expectedErr := errors.New("review failed")
		runHuhForm = func(*huh.Form) error { return nil }
		runWizardReviewStep = func(io.Reader, io.Writer, wizardState) (bool, error) {
			return false, expectedErr
		}

		if _, err := runHuhWizardWithPlatform(strings.NewReader(""), io.Discard, base, platform); !errors.Is(err, expectedErr) {
			t.Fatalf("expected review error %v, got %v", expectedErr, err)
		}
	})

	t.Run("config conversion error", func(t *testing.T) {
		originalRunForm := runHuhForm
		originalRunReview := runHuhReviewForm
		originalReviewStep := runWizardReviewStep
		originalToConfig := wizardStateToConfigForPlatform
		t.Cleanup(func() {
			runHuhForm = originalRunForm
			runHuhReviewForm = originalRunReview
			runWizardReviewStep = originalReviewStep
			wizardStateToConfigForPlatform = originalToConfig
		})

		expectedErr := errors.New("convert failed")
		runHuhForm = func(*huh.Form) error { return nil }
		runWizardReviewStep = func(io.Reader, io.Writer, wizardState) (bool, error) { return true, nil }
		wizardStateToConfigForPlatform = func(wizardState, Config, system.Platform) (Config, error) {
			return Config{}, expectedErr
		}

		if _, err := runHuhWizardWithPlatform(strings.NewReader(""), io.Discard, base, platform); !errors.Is(err, expectedErr) {
			t.Fatalf("expected conversion error %v, got %v", expectedErr, err)
		}
	})

	t.Run("review rejection returns wizard cancelled", func(t *testing.T) {
		originalRunForm := runHuhForm
		originalRunReview := runHuhReviewForm
		originalReviewStep := runWizardReviewStep
		originalToConfig := wizardStateToConfigForPlatform
		t.Cleanup(func() {
			runHuhForm = originalRunForm
			runHuhReviewForm = originalRunReview
			runWizardReviewStep = originalReviewStep
			wizardStateToConfigForPlatform = originalToConfig
		})

		runHuhForm = func(*huh.Form) error { return nil }
		runWizardReviewStep = func(io.Reader, io.Writer, wizardState) (bool, error) { return false, nil }

		_, err := runHuhWizardWithPlatform(strings.NewReader(""), &bytes.Buffer{}, base, platform)
		if err == nil || !strings.Contains(err.Error(), "wizard cancelled") {
			t.Fatalf("expected wizard cancellation, got %v", err)
		}
	})
}

func TestRunWizardReviewSurfacesInjectedRunErrors(t *testing.T) {
	originalRunReview := runHuhReviewForm
	t.Cleanup(func() { runHuhReviewForm = originalRunReview })

	expectedErr := errors.New("review run failed")
	runHuhReviewForm = func(*huh.Form) error { return expectedErr }

	if _, err := runWizardReview(strings.NewReader(""), io.Discard, newWizardState(Default())); !errors.Is(err, expectedErr) {
		t.Fatalf("expected review run error %v, got %v", expectedErr, err)
	}
}
