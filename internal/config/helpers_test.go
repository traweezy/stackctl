package config

import (
	"strings"
	"testing"
)

func TestPackageManagerWizardSuggestionsNormalizesAndDeduplicates(t *testing.T) {
	values := packageManagerWizardSuggestions(" APT ")
	if len(values) == 0 {
		t.Fatal("expected package-manager suggestions")
	}

	seen := map[string]struct{}{}
	for _, value := range values {
		if value != strings.ToLower(strings.TrimSpace(value)) {
			t.Fatalf("expected normalized suggestion, got %q", value)
		}
		if _, ok := seen[value]; ok {
			t.Fatalf("unexpected duplicate suggestion %q in %+v", value, values)
		}
		seen[value] = struct{}{}
	}

	for _, expected := range []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "brew"} {
		if _, ok := seen[expected]; !ok {
			t.Fatalf("expected suggestion %q in %+v", expected, values)
		}
	}
}

func TestSelectedStackNameFallsBackToDefaultOnInvalidEnv(t *testing.T) {
	t.Setenv(StackNameEnvVar, "INVALID!")

	if got := SelectedStackName(); got != DefaultStackName {
		t.Fatalf("unexpected selected stack name: %q", got)
	}
}

func TestWizardHelperFunctions(t *testing.T) {
	state := newWizardState(Default())
	state.Services = []string{"postgres", "meilisearch"}
	state.StackMode = wizardStackModeManaged

	if got := state.stackModeLabel(); got != "Managed" {
		t.Fatalf("unexpected stack mode label: %q", got)
	}

	displayNames := state.serviceDisplayNames()
	if strings.Join(displayNames, ",") != "Postgres,Meilisearch" {
		t.Fatalf("unexpected service display names: %+v", displayNames)
	}

	options := serviceOptions(&state)
	if len(options) != 6 {
		t.Fatalf("unexpected service option count: %d", len(options))
	}
	if options[0].Key != "Postgres" || options[0].Value != "postgres" {
		t.Fatalf("unexpected first service option: %+v", options[0])
	}
	if options[4].Key != "Meilisearch" || options[4].Value != "meilisearch" {
		t.Fatalf("unexpected meilisearch option: %+v", options[4])
	}

	position, total := wizardStepPosition(&state, wizardStepReview)
	if position == 0 || total == 0 || position > total {
		t.Fatalf("unexpected wizard step position: position=%d total=%d", position, total)
	}
	if got := wizardStepLabel(wizardStepReview); got == "Setup" {
		t.Fatalf("expected a specific review label, got %q", got)
	}
	if next := wizardNextStepLabel(&state, wizardStepReview); next != "" {
		t.Fatalf("expected no next step after review, got %q", next)
	}
}

func TestValidationTextHelpers(t *testing.T) {
	if err := validPortText("5432"); err != nil {
		t.Fatalf("validPortText returned error: %v", err)
	}
	if err := validPortText("70000"); err == nil {
		t.Fatal("expected invalid port text to fail")
	}

	if err := validPositiveIntText("42"); err != nil {
		t.Fatalf("validPositiveIntText returned error: %v", err)
	}
	if err := validPositiveIntText("0"); err == nil {
		t.Fatal("expected non-positive integer to fail")
	}

	if value, err := parsePostgresLogDurationMS("-1"); err != nil || value != -1 {
		t.Fatalf("unexpected postgres log duration parse: value=%d err=%v", value, err)
	}
	if err := validPostgresLogDurationText("1500"); err != nil {
		t.Fatalf("validPostgresLogDurationText returned error: %v", err)
	}
	if err := validPostgresLogDurationText("0"); err == nil {
		t.Fatal("expected invalid postgres log duration to fail")
	}
}
