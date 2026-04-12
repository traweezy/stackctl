package tui

import (
	"strings"
	"testing"

	"github.com/traweezy/stackctl/internal/output"
)

func TestInspectSelectionHelpersCoverFallbacks(t *testing.T) {
	snapshot := Snapshot{
		Services: []Service{
			{Name: "postgres", DisplayName: "Postgres"},
			{Name: "", DisplayName: "Cockpit Dashboard"},
			{Name: "", DisplayName: "!!!"},
		},
		Stacks: []StackProfile{
			{Name: "dev-stack", Current: true},
			{Name: ""},
			{Name: "observability"},
		},
	}

	if got := serviceKey(snapshot.Services[1]); got != "cockpitdashboard" {
		t.Fatalf("unexpected fallback service key %q", got)
	}
	if got := serviceKey(snapshot.Services[2]); got != "" {
		t.Fatalf("expected punctuation-only display names to produce empty keys, got %q", got)
	}

	if got := selectableServiceNames(snapshot); len(got) != 2 || got[1] != "cockpitdashboard" {
		t.Fatalf("unexpected selectable service names %+v", got)
	}
	if got := selectableStackNames(snapshot); len(got) != 2 || got[1] != "observability" {
		t.Fatalf("unexpected selectable stack names %+v", got)
	}
	if got := pickSelectedName("missing", []string{"postgres", "redis"}); got != "postgres" {
		t.Fatalf("expected missing selection to fall back to the first value, got %q", got)
	}
	if got := cycleSelectedName("", nil, 1); got != "" {
		t.Fatalf("expected empty available list to stay blank, got %q", got)
	}

	service, ok := selectedService(snapshot, "missing")
	if !ok || service.DisplayName != "Postgres" {
		t.Fatalf("expected selectedService fallback, got service=%+v ok=%v", service, ok)
	}
	if _, ok := selectedService(Snapshot{}, "postgres"); ok {
		t.Fatal("expected selectedService to fail for empty snapshots")
	}

	profile, ok := selectedStackProfile(snapshot, "missing")
	if !ok || profile.Name != "dev-stack" {
		t.Fatalf("expected selectedStackProfile fallback, got profile=%+v ok=%v", profile, ok)
	}
	if _, ok := selectedStackProfile(Snapshot{}, "dev-stack"); ok {
		t.Fatal("expected selectedStackProfile to fail for empty snapshots")
	}
}

func TestInspectRenderersCoverEmptyRawAndCategorizedBranches(t *testing.T) {
	if got := renderStacks(Snapshot{}, "", expandedLayout, 120); !strings.Contains(got, "No stack profiles are available.") {
		t.Fatalf("expected empty stacks state, got:\n%s", got)
	}

	serviceErrorSnapshot := Snapshot{
		ServiceError: "service load failed",
	}
	if got := renderServices(serviceErrorSnapshot, false, expandedLayout, "", 120, nil); !strings.Contains(got, "No services loaded.") || !strings.Contains(got, "service load failed") {
		t.Fatalf("expected empty services state with error banner, got:\n%s", got)
	}

	rawHealth := Snapshot{
		Health: []HealthLine{{Status: output.StatusWarn, Message: "redis is warming up"}},
		DoctorChecks: []DoctorCheck{
			{Status: output.StatusOK, Message: "podman looks good"},
			{Status: output.StatusWarn, Message: "compose provider is old"},
		},
		DoctorSummary: DoctorSummary{OK: 1, Warn: 1},
	}
	healthView := renderHealth(rawHealth, "", 120, nil)
	for _, fragment := range []string{
		"Live service health is unavailable; showing raw checks instead.",
		"redis is warming up",
		"Doctor findings",
		"compose provider is old",
	} {
		if !strings.Contains(healthView, fragment) {
			t.Fatalf("expected raw health view to contain %q:\n%s", fragment, healthView)
		}
	}

	services := []Service{
		{Name: "postgres", DisplayName: "Postgres", Status: "running", ContainerName: "local-postgres", ExternalPort: 5432},
		{Name: "redis", DisplayName: "Redis", Status: "missing", ContainerName: "local-redis", ExternalPort: 6379},
		{Name: "cockpit", DisplayName: "Cockpit", Status: "running", URL: "https://localhost:9090"},
	}
	pinned := map[string]struct{}{"postgres": {}}
	healthSnapshot := Snapshot{
		Services:      services,
		DoctorChecks:  []DoctorCheck{{Status: output.StatusWarn, Message: "manual follow-up required"}},
		DoctorSummary: DoctorSummary{Warn: 1},
		HealthError:   "health probe lag",
		DoctorError:   "doctor report stale",
	}

	listPane := renderHealthListPane(healthSnapshot, "postgres", pinned)
	for _, fragment := range []string{
		"Pinned",
		"Stack services",
		"Host tools",
		"Postgres",
		"Redis",
		"Cockpit",
	} {
		if !strings.Contains(listPane, fragment) {
			t.Fatalf("expected health list pane to contain %q:\n%s", fragment, listPane)
		}
	}

	if got := renderHealth(healthSnapshot, "missing", 120, pinned); !strings.Contains(got, "Health detail") || !strings.Contains(got, "Postgres") {
		t.Fatalf("expected missing selections to fall back to the first health target, got:\n%s", got)
	}

	footer := renderHealthDoctorFooter(Snapshot{DoctorSummary: DoctorSummary{}}, 100)
	if footer != "" {
		t.Fatalf("expected empty footer for empty doctor state, got %q", footer)
	}

	noFindingsFooter := renderHealthDoctorFooter(Snapshot{DoctorSummary: DoctorSummary{OK: 2}}, 100)
	if !strings.Contains(noFindingsFooter, "No doctor findings recorded.") {
		t.Fatalf("expected no-findings footer copy, got:\n%s", noFindingsFooter)
	}

	if got := doctorFooterColor(DoctorSummary{Fail: 1}); got != "160" {
		t.Fatalf("unexpected fail doctor footer color %q", got)
	}
	if got := doctorFooterColor(DoctorSummary{Warn: 1}); got != "221" {
		t.Fatalf("unexpected warn doctor footer color %q", got)
	}
	if got := doctorFooterColor(DoctorSummary{OK: 1}); got != "28" {
		t.Fatalf("unexpected ok doctor footer color %q", got)
	}

	hostHint := stripANSITest(renderProductivityHint(services[2], nil))
	if strings.Contains(hostHint, "d db shell") || strings.Contains(hostHint, "w logs") {
		t.Fatalf("did not expect host-tool productivity hint to expose stack-only actions, got %q", hostHint)
	}

	stackHint := stripANSITest(renderProductivityHint(services[0], pinned))
	for _, fragment := range []string{"c copy", "w logs", "e shell", "d db shell", "p unpin"} {
		if !strings.Contains(stackHint, fragment) {
			t.Fatalf("expected stack productivity hint to contain %q, got %q", fragment, stackHint)
		}
	}
}
