package config

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"charm.land/huh/v2"

	"github.com/traweezy/stackctl/internal/system"
)

func TestWizardStepNoteDynamicCallbacksCoverVisibleHiddenAndFinalBranches(t *testing.T) {
	platform := system.Platform{
		GOOS:           "linux",
		PackageManager: "dnf",
		ServiceManager: system.ServiceManagerSystemd,
	}
	state := newWizardState(DefaultForStackOnPlatform("dev-stack", platform))

	stackNote := wizardStepNote(&state, wizardStepStack)
	stackTitle := wizardEvalStringFunc(t, stackNote, "title")
	stackDescription := wizardEvalStringFunc(t, stackNote, "description")
	if got := stackTitle(); !strings.Contains(got, "Step 1 of") || !strings.Contains(got, "Stack") {
		t.Fatalf("unexpected stack note title: %q", got)
	}
	if got := stackDescription(); !strings.Contains(got, "Next:") {
		t.Fatalf("expected stack note to advertise the next step, got %q", got)
	}

	hiddenExternal := wizardStepNote(&state, wizardStepExternalStack)
	if got := wizardEvalStringFunc(t, hiddenExternal, "title")(); strings.Contains(got, "Step ") || !strings.Contains(got, "External stack target") {
		t.Fatalf("expected hidden external step to fall back to its label, got %q", got)
	}

	state.Services = []string{"postgres", "redis", "nats", "seaweedfs", "meilisearch", "pgadmin"}
	state.IncludeCockpit = true
	finalNote := wizardStepNote(&state, wizardStepReview)
	if got := wizardEvalStringFunc(t, finalNote, "description")(); got != "Final confirmation before the config is written." {
		t.Fatalf("unexpected final-step description: %q", got)
	}
}

func TestBuildWizardFormHideFuncsAndDynamicDescriptions(t *testing.T) {
	platform := system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	}
	state := newWizardState(DefaultForStackOnPlatform("dev-stack", platform))
	state.Services = []string{"postgres", "redis", "nats", "seaweedfs", "meilisearch", "pgadmin"}
	state.IncludeCockpit = true

	form := buildWizardForm(&state, platform)
	groups := wizardGroupsByTitle(t, form)

	if hide := wizardGroupHideFunc(t, groups["External Stack"]); hide == nil || !hide() {
		t.Fatal("expected the external-stack group to stay hidden for managed stacks")
	}
	if hide := wizardGroupHideFunc(t, groups["External Path"]); hide == nil || !hide() {
		t.Fatal("expected the missing-path confirmation group to stay hidden by default")
	}
	for _, title := range []string{"Postgres", "Redis", "NATS", "SeaweedFS", "Meilisearch", "pgAdmin", "Cockpit Settings"} {
		hide := wizardGroupHideFunc(t, groups[title])
		if hide == nil || hide() {
			t.Fatalf("expected %s group to stay visible when enabled", title)
		}
	}

	state.StackMode = wizardStackModeExternal
	state.ExternalStackDir = filepath.Join(t.TempDir(), "missing-external-stack")
	state.ExternalComposeFile = "compose.external.yaml"
	state.Services = []string{"postgres"}
	state.IncludeCockpit = false

	if hide := wizardGroupHideFunc(t, groups["External Stack"]); hide == nil || hide() {
		t.Fatal("expected the external-stack group to show for external stacks")
	}
	if hide := wizardGroupHideFunc(t, groups["External Path"]); hide == nil || hide() {
		t.Fatal("expected the missing external path confirmation to show for a missing directory")
	}
	if hide := wizardGroupHideFunc(t, groups["Redis"]); hide == nil || !hide() {
		t.Fatal("expected the redis group to hide when redis is not selected")
	}
	if hide := wizardGroupHideFunc(t, groups["Cockpit Settings"]); hide == nil || !hide() {
		t.Fatal("expected cockpit settings to hide when cockpit helpers are disabled")
	}

	externalPathFields := wizardGroupFields(t, groups["External Path"])
	confirm, ok := externalPathFields[1].(*huh.Confirm)
	if !ok {
		t.Fatalf("expected external path group to contain a confirm field, got %T", externalPathFields[1])
	}
	description := wizardEvalStringFunc(t, confirm, "description")
	if got := description(); !strings.Contains(got, state.ExternalStackDir) || !strings.Contains(got, "does not exist yet") {
		t.Fatalf("unexpected external-path confirmation description: %q", got)
	}
}

func TestNewWizardStateAndParsePortCoverRemainingBranches(t *testing.T) {
	cfg := Default()
	cfg.Setup.IncludeRedis = false
	cfg.Setup.IncludeNATS = false
	cfg.Setup.IncludeSeaweedFS = true
	cfg.Setup.IncludeMeilisearch = true
	cfg.Setup.IncludePgAdmin = false
	cfg.Setup.IncludeCockpit = false
	cfg.Setup.InstallCockpit = true

	state := newWizardState(cfg)
	if strings.Join(state.Services, ",") != "postgres,seaweedfs,meilisearch" {
		t.Fatalf("unexpected enabled services: %v", state.Services)
	}
	if state.IncludeCockpit {
		t.Fatalf("expected cockpit helpers to stay disabled, got %+v", state)
	}

	if _, err := parsePort("   "); err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("expected blank port error, got %v", err)
	}
	if _, err := parsePort("abc"); err == nil || !strings.Contains(err.Error(), "valid number") {
		t.Fatalf("expected invalid port error, got %v", err)
	}
	if _, err := parsePort("70000"); err == nil || !strings.Contains(err.Error(), "between 1 and 65535") {
		t.Fatalf("expected out-of-range port error, got %v", err)
	}
	port, err := parsePort(" 5432 ")
	if err != nil || port != 5432 {
		t.Fatalf("expected trimmed port parse, got %d err=%v", port, err)
	}
}

func wizardGroupsByTitle(t *testing.T, form *huh.Form) map[string]*huh.Group {
	t.Helper()

	groups := make(map[string]*huh.Group)
	for _, group := range wizardFormGroups(t, form) {
		title := wizardHiddenField(t, reflect.ValueOf(group), "title").String()
		groups[title] = group
	}
	return groups
}

func wizardFormGroups(t *testing.T, form *huh.Form) []*huh.Group {
	t.Helper()

	selector := wizardHiddenField(t, reflect.ValueOf(form), "selector")
	items := wizardHiddenField(t, selector, "items")
	groups := make([]*huh.Group, items.Len())
	for idx := range groups {
		groups[idx] = items.Index(idx).Interface().(*huh.Group)
	}
	return groups
}

func wizardGroupFields(t *testing.T, group *huh.Group) []huh.Field {
	t.Helper()

	selector := wizardHiddenField(t, reflect.ValueOf(group), "selector")
	items := wizardHiddenField(t, selector, "items")
	fields := make([]huh.Field, items.Len())
	for idx := range fields {
		fields[idx] = items.Index(idx).Interface().(huh.Field)
	}
	return fields
}

func wizardGroupHideFunc(t *testing.T, group *huh.Group) func() bool {
	t.Helper()

	hide := wizardHiddenField(t, reflect.ValueOf(group), "hide")
	if hide.IsNil() {
		return nil
	}
	return hide.Interface().(func() bool)
}

func wizardEvalStringFunc(t *testing.T, target any, name string) func() string {
	t.Helper()

	eval := wizardHiddenField(t, reflect.ValueOf(target), name)
	fn := wizardHiddenField(t, eval, "fn")
	if fn.IsNil() {
		t.Fatalf("expected %T.%s to have a dynamic callback", target, name)
	}
	return fn.Interface().(func() string)
}

func wizardHiddenField(t *testing.T, value reflect.Value, name string) reflect.Value {
	t.Helper()

	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			t.Fatalf("nil pointer while reading %s", name)
		}
		value = value.Elem()
	}

	field := value.FieldByName(name)
	if !field.IsValid() {
		t.Fatalf("field %s not found on %s", name, value.Type())
	}

	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}
