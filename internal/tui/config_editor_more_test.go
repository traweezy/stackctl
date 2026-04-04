package tui

import (
	"errors"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

func testConfigSpecByKey(t *testing.T, key string) configFieldSpec {
	t.Helper()

	for _, spec := range configFieldSpecs {
		if spec.Key == key {
			return spec
		}
	}

	t.Fatalf("missing config field spec for %s", key)
	return configFieldSpec{}
}

func TestConfigEditorPreviewHelpersCoverEmptyDiffChangesAndMarshalErrors(t *testing.T) {
	cfg := configpkg.Default()
	editor := newConfigEditor()
	editor.source = ConfigSourceLoaded
	editor.baseline = cfg
	editor.draft = cfg

	preview := stripANSITest(editor.renderPreview(false))
	if !strings.Contains(preview, "No unsaved config changes to preview.") || !strings.Contains(preview, "Loaded from disk.") {
		t.Fatalf("expected empty preview state, got:\n%s", preview)
	}

	editor.draft.Connection.Host = "db.internal"
	editor.draft.ApplyDerivedFields()
	preview = stripANSITest(editor.renderPreview(false))
	if strings.Contains(preview, "No unsaved config changes to preview.") || !strings.Contains(preview, "db.internal") {
		t.Fatalf("expected diff preview for changed host, got:\n%s", preview)
	}

	editor.draft = cfg
	editor.draft.Stack.Name = "INVALID!"
	preview = stripANSITest(editor.renderPreview(false))
	if !strings.Contains(preview, "INVALID!") {
		t.Fatalf("expected preview diff to include the updated stack name, got:\n%s", preview)
	}
}

func TestConfigEditorFieldFormattingAndStateHelpers(t *testing.T) {
	cfg := configpkg.Default()
	secretSpec := testConfigSpecByKey(t, "connection.postgres_password")
	policySpec := testConfigSpecByKey(t, "services.redis.maxmemory_policy")
	stackNameSpec := testConfigSpecByKey(t, "stack.name")
	volumeLimitSpec := testConfigSpecByKey(t, "services.seaweedfs.volume_size_limit_mb")

	if got := displayFieldValue(secretSpec, cfg, false); got != maskedSecret {
		t.Fatalf("expected masked secret value, got %q", got)
	}
	if got := displayFieldValue(secretSpec, cfg, true); got != cfg.Connection.PostgresPassword {
		t.Fatalf("expected plain secret value, got %q", got)
	}
	if got := compactFieldValue(secretSpec, cfg, false); got != maskedSecret {
		t.Fatalf("expected compact secret to stay masked, got %q", got)
	}
	if got := compactFieldValue(policySpec, cfg, true); !strings.Contains(got, "noeviction") {
		t.Fatalf("expected compact field value to include policy, got %q", got)
	}

	if got := truncateMiddle("abcdefghijklmnopqrstuvwxyz", 9); got != "abcd…wxyz" {
		t.Fatalf("unexpected truncateMiddle result %q", got)
	}
	if got := truncateEnd("abcdef", 4); got != "abc…" {
		t.Fatalf("unexpected truncateEnd result %q", got)
	}
	if got := titleCaseLabel("  stackctl"); got != "Stackctl" {
		t.Fatalf("unexpected titleCaseLabel result %q", got)
	}
	if got := wrapText("one two three four five", 10); got != "one two three four five" {
		t.Fatalf("expected wrapText to skip very narrow wrapping, got %q", got)
	}
	if got := wrapText("one two three four five", 20); !strings.Contains(got, "\n") {
		t.Fatalf("expected wrapText to wrap once the width is large enough, got %q", got)
	}
	if got := wrapText("one two", 12); got != "one two" {
		t.Fatalf("expected wrapText to keep short content on one line, got %q", got)
	}

	if got := policySpec.suggestionHeading(); got != "Redis policies" {
		t.Fatalf("unexpected custom suggestion heading %q", got)
	}
	if got := stackNameSpec.suggestionHeading(); got != "Suggested values" {
		t.Fatalf("unexpected default suggestion heading %q", got)
	}
	if got := selectedFieldValue(configFieldSpec{}, cfg); got != "" {
		t.Fatalf("expected missing getter to return empty string, got %q", got)
	}

	editor := newConfigEditor()
	editor.baseline = cfg
	editor.draft = cfg
	editor.selectedKey = "stack.name"

	editor.editing = true
	editor.input.Err = errors.New("invalid")
	if got := editor.configFieldState(stackNameSpec); got != "invalid" {
		t.Fatalf("expected editing error state to be invalid, got %q", got)
	}

	editor.input.Err = nil
	if got := editor.configFieldState(stackNameSpec); got != "editing" {
		t.Fatalf("expected active edit state, got %q", got)
	}

	editor.editing = false
	editor.issueIndex = map[string][]configpkg.ValidationIssue{
		"stack.name": {{Field: "stack.name", Message: "invalid"}},
	}
	if got := editor.configFieldState(stackNameSpec); got != "invalid" {
		t.Fatalf("expected indexed issue state, got %q", got)
	}

	editor.issueIndex = nil
	editor.source = ConfigSourceMissing
	if got := editor.configFieldState(stackNameSpec); got != "edited" {
		t.Fatalf("expected missing-source state to be edited, got %q", got)
	}

	editor.source = ConfigSourceLoaded
	editor.draft.Stack.Name = "dev-stack-ops"
	if got := editor.configFieldState(stackNameSpec); got != "edited" {
		t.Fatalf("expected changed field state to be edited, got %q", got)
	}

	editor.draft = cfg
	if got := editor.configFieldState(stackNameSpec); got != "clean" {
		t.Fatalf("expected unchanged field state to be clean, got %q", got)
	}
	if got := configFieldStateLabel("editing"); got != "editing" {
		t.Fatalf("unexpected state label %q", got)
	}
	if got := stripANSITest(configFieldStateChip("edited")); !strings.Contains(got, "edited") {
		t.Fatalf("expected edited state chip to include label, got %q", got)
	}
	if got := configFieldStateChip("clean"); got != "" {
		t.Fatalf("expected clean state chip to be blank, got %q", got)
	}
	if got := stripANSITest(editor.renderConfigFieldHeading(volumeLimitSpec)); !strings.Contains(got, "SeaweedFS / Volume size limit") {
		t.Fatalf("unexpected config field heading %q", got)
	}

	if err := requiredText(cfg, "   "); err == nil {
		t.Fatal("expected requiredText to reject blanks")
	}
	if err := validStackNameText(cfg, "Invalid!"); err == nil {
		t.Fatal("expected validStackNameText to reject invalid names")
	}
	if err := positiveIntText(cfg, "zero"); err == nil {
		t.Fatal("expected positiveIntText to reject non-numeric input")
	}
	if err := positiveIntText(cfg, "0"); err == nil {
		t.Fatal("expected positiveIntText to reject zero")
	}
}

func TestConfigEditorSummaryAndFollowUpHelpersCoverRepresentativeBranches(t *testing.T) {
	editor := newConfigEditor()
	editor.draft = configpkg.Default()

	editor.editing = true
	editor.input.Err = errors.New("invalid")
	if got := editor.compactNextStepSummary(configApplyPlan{}); got != "ctrl+s is blocked until this field is fixed" {
		t.Fatalf("unexpected invalid-edit summary %q", got)
	}

	editor.input.Err = nil
	if got := editor.compactNextStepSummary(configApplyPlan{}); got != "press Enter to keep this edit in the draft" {
		t.Fatalf("unexpected editing summary %q", got)
	}

	editor.editing = false
	editor.issues = []configpkg.ValidationIssue{{Field: "stack.name", Message: "invalid"}}
	if got := editor.compactNextStepSummary(configApplyPlan{}); got != "fix 1 validation issue(s) before ctrl+s" {
		t.Fatalf("unexpected issue summary %q", got)
	}

	editor.issues = nil
	if got := editor.compactNextStepSummary(configApplyPlan{Allowed: true, Restart: true}); got != "ctrl+s saves, refreshes compose, and restarts running services" {
		t.Fatalf("unexpected restart summary %q", got)
	}
	if got := editor.compactNextStepSummary(configApplyPlan{Allowed: true, Scaffold: true}); got != "ctrl+s saves and refreshes compose for the next start" {
		t.Fatalf("unexpected scaffold summary %q", got)
	}
	if got := editor.compactNextStepSummary(configApplyPlan{Reason: "stack target changes are save-only"}); got != "ctrl+s saves only; stack target changes do not restart the current stack" {
		t.Fatalf("unexpected stack-target summary %q", got)
	}

	editor.draft.Stack.Managed = false
	if got := editor.compactNextStepSummary(configApplyPlan{Reason: "use ctrl+s to save config-only changes"}); got != "ctrl+s writes config only; external compose stays unchanged" {
		t.Fatalf("unexpected external config-only summary %q", got)
	}
	if got := editor.compactNextStepSummary(configApplyPlan{Reason: "nothing new needs to be applied"}); got != "ctrl+s would only update stackctl metadata here" {
		t.Fatalf("unexpected external no-op summary %q", got)
	}

	editor.draft.Stack.Managed = true
	if got := editor.compactNextStepSummary(configApplyPlan{Reason: "managed compose changes require manual compose updates"}); got != "ctrl+s saves only; update compose manually after" {
		t.Fatalf("unexpected manual compose summary %q", got)
	}
	if got := editor.compactNextStepSummary(configApplyPlan{Reason: "resolve the managed scaffold problem before applying"}); got != "fix the scaffold problem before ctrl+s" {
		t.Fatalf("unexpected scaffold-problem summary %q", got)
	}
	if got := editor.compactNextStepSummary(configApplyPlan{}); got != "ctrl+s saves this draft" {
		t.Fatalf("unexpected default summary %q", got)
	}

	previous := configpkg.DefaultForStack("dev-stack")
	if got := saveFollowUpMessage(previous, previous, 0); got != "running services are unchanged" {
		t.Fatalf("unexpected unchanged follow-up %q", got)
	}

	managedCompose := previous
	managedCompose.Services.Postgres.Image = "docker.io/library/postgres:18"
	managedCompose.ApplyDerivedFields()
	if got := saveFollowUpMessage(previous, managedCompose, 0); got != "save also needs a compose refresh before the next start" {
		t.Fatalf("unexpected managed compose follow-up %q", got)
	}

	manualManaged := previous
	manualManaged.Setup.ScaffoldDefaultStack = false
	manualManaged.Services.Postgres.Image = "docker.io/library/postgres:18"
	manualManaged.ApplyDerivedFields()
	if got := saveFollowUpMessage(previous, manualManaged, 0); got != "update the managed compose file yourself before restart" {
		t.Fatalf("unexpected manual managed follow-up %q", got)
	}

	externalPrevious := previous
	externalPrevious.Stack.Managed = false
	externalPrevious.Setup.ScaffoldDefaultStack = false
	externalPrevious.Stack.Dir = "/tmp/dev-stack"
	externalPrevious.Stack.ComposeFile = "compose.yaml"
	externalPrevious.ApplyDerivedFields()

	externalNext := externalPrevious
	externalNext.Services.Postgres.Image = "docker.io/library/postgres:18"
	externalNext.ApplyDerivedFields()
	if got := saveFollowUpMessage(externalPrevious, externalNext, 0); got != "external compose files were not changed" {
		t.Fatalf("unexpected external compose follow-up %q", got)
	}
}

func TestScaffoldConfigCmdCoversSaveFailureScaffoldFailureAndSuccessMessages(t *testing.T) {
	cfg := configpkg.DefaultForStack("dev-stack")

	t.Run("save failure", func(t *testing.T) {
		saveErr := errors.New("disk full")
		cmd := scaffoldConfigCmd(&ConfigManager{
			SaveConfig: func(string, configpkg.Config) error { return saveErr },
		}, "/tmp/config.yaml", cfg, false, 0)
		msg := msgOfType[configOperationMsg](t, cmd)
		if msg.Status != output.StatusFail || !strings.Contains(msg.Message, "save config failed: disk full") || !errors.Is(msg.Err, saveErr) {
			t.Fatalf("unexpected save-failure message %+v", msg)
		}
	})

	t.Run("scaffold failure", func(t *testing.T) {
		cmd := scaffoldConfigCmd(&ConfigManager{
			SaveConfig: func(string, configpkg.Config) error { return nil },
			ScaffoldManagedStack: func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) {
				return configpkg.ScaffoldResult{}, errors.New("template failed")
			},
		}, "/tmp/config.yaml", cfg, false, 0)
		msg := msgOfType[configOperationMsg](t, cmd)
		if msg.Status != output.StatusFail || !strings.Contains(msg.Message, "scaffold failed: template failed") || msg.Err == nil {
			t.Fatalf("unexpected scaffold-failure message %+v", msg)
		}
	})

	t.Run("up-to-date scaffold message includes restart hint", func(t *testing.T) {
		cmd := scaffoldConfigCmd(&ConfigManager{
			SaveConfig:           func(string, configpkg.Config) error { return nil },
			ScaffoldManagedStack: func(configpkg.Config, bool) (configpkg.ScaffoldResult, error) { return configpkg.ScaffoldResult{}, nil },
		}, "/tmp/config.yaml", cfg, false, 1)
		msg := msgOfType[configOperationMsg](t, cmd)
		if msg.Status != output.StatusOK || !msg.Reload {
			t.Fatalf("unexpected scaffold success status %+v", msg)
		}
		for _, fragment := range []string{
			"managed stack scaffold is up to date",
			"restart the stack to apply updated compose changes",
		} {
			if !strings.Contains(msg.Message, fragment) {
				t.Fatalf("expected scaffold success message to contain %q, got %q", fragment, msg.Message)
			}
		}
	})

	result := configpkg.ScaffoldResult{ComposePath: "/tmp/dev-stack/compose.yaml", AlreadyPresent: true}
	if got := scaffoldResultMessage(result); got != "managed stack already exists at /tmp/dev-stack/compose.yaml" {
		t.Fatalf("unexpected already-present scaffold message %q", got)
	}
}
