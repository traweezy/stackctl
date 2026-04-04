package config

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty/v2"

	"github.com/traweezy/stackctl/internal/system"
)

func TestRunWizardUsesForcedPlainMode(t *testing.T) {
	t.Setenv("STACKCTL_WIZARD_PLAIN", "1")

	base := DefaultForStackOnPlatform("dev-stack", system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	})
	base.Setup.IncludeRedis = false
	base.Setup.IncludeNATS = false
	base.Setup.IncludeSeaweedFS = false
	base.Setup.IncludeMeilisearch = false
	base.Setup.IncludePgAdmin = false
	base.Setup.IncludeCockpit = false

	input := wizardAnswers(
		"", // stack name
		"", // manage stack
		"", // include postgres
		"", // postgres container
		"", // postgres image
		"", // postgres data volume
		"", // postgres maintenance database
		"", // postgres port
		"", // postgres database
		"", // postgres username
		"", // postgres password
		"", // include redis
		"", // include nats
		"", // include seaweedfs
		"", // include meilisearch
		"", // include pgadmin
		"", // include cockpit
		"", // wait for services
		"", // startup timeout
		"", // package manager
	)

	cfg, err := RunWizard(strings.NewReader(input), io.Discard, base)
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}
	if !cfg.Stack.Managed || !cfg.Setup.ScaffoldDefaultStack {
		t.Fatalf("expected managed defaults to be preserved, got %+v", cfg.Stack)
	}
	if !cfg.Setup.IncludePostgres || cfg.Setup.IncludeRedis || cfg.Setup.IncludeCockpit {
		t.Fatalf("unexpected service selection: %+v", cfg.Setup)
	}
	if cfg.System.PackageManager != "apt" {
		t.Fatalf("expected package manager default to be preserved, got %q", cfg.System.PackageManager)
	}
}

func TestShouldUsePlainWizardReturnsFalseForPTY(t *testing.T) {
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

	if shouldUsePlainWizard(inTTY, outTTY) {
		t.Fatal("expected PTY-backed stdin/stdout to keep the rich wizard path")
	}
}

func TestRunPlainWizardWithPlatformCoversCustomExternalServiceFlow(t *testing.T) {
	existingDir := t.TempDir()
	missingDir := filepath.Join(t.TempDir(), "future-stack")
	platform := system.Platform{
		GOOS:           "darwin",
		PackageManager: "brew",
		ServiceManager: system.ServiceManagerNone,
	}

	base := DefaultForStackOnPlatform("dev-stack", platform)
	base.Stack.Managed = false
	base.Setup.ScaffoldDefaultStack = false
	base.Stack.Dir = existingDir
	base.Stack.ComposeFile = "compose.yaml"
	base.Setup.IncludePostgres = false
	base.Setup.IncludeRedis = false
	base.Setup.IncludeNATS = false
	base.Setup.IncludeSeaweedFS = false
	base.Setup.IncludeMeilisearch = false
	base.Setup.IncludePgAdmin = false
	base.Setup.IncludeCockpit = false

	var out bytes.Buffer
	cfg, err := runPlainWizardWithPlatform(strings.NewReader(wizardAnswers(
		"custom-stack",
		"n",
		missingDir,
		"n",
		existingDir,
		"compose.dev.yaml",
		"n",
		"n",
		"n",
		"y",
		"custom-seaweed",
		"chrislusf/seaweedfs:latest",
		"custom_seaweed_data",
		"oops",
		"2048",
		"18333",
		"weed-access",
		"weed-secret",
		"y",
		"custom-meili",
		"getmeili/meilisearch:v1.16.0",
		"custom_meili_data",
		"17700",
		"short",
		"1234567890abcdef",
		"n",
		"y",
		"19090",
		"",
		"45",
		"",
	)), &out, base, platform)
	if err != nil {
		t.Fatalf("runPlainWizardWithPlatform returned error: %v", err)
	}

	if cfg.Stack.Name != "custom-stack" {
		t.Fatalf("unexpected stack name %q", cfg.Stack.Name)
	}
	if cfg.Stack.Managed || cfg.Setup.ScaffoldDefaultStack {
		t.Fatalf("expected external stack settings, got managed=%v scaffold=%v", cfg.Stack.Managed, cfg.Setup.ScaffoldDefaultStack)
	}
	if cfg.Stack.Dir != existingDir {
		t.Fatalf("unexpected stack dir %q", cfg.Stack.Dir)
	}
	if cfg.Stack.ComposeFile != "compose.dev.yaml" {
		t.Fatalf("unexpected compose file %q", cfg.Stack.ComposeFile)
	}
	if !cfg.Setup.IncludeSeaweedFS || !cfg.Setup.IncludeMeilisearch || !cfg.Setup.IncludeCockpit {
		t.Fatalf("expected selected services to be enabled, got %+v", cfg.Setup)
	}
	if cfg.Setup.InstallCockpit {
		t.Fatalf("expected unsupported cockpit install to stay disabled, got %+v", cfg.Setup)
	}
	if cfg.Services.SeaweedFS.VolumeSizeLimitMB != 2048 {
		t.Fatalf("unexpected SeaweedFS limit %d", cfg.Services.SeaweedFS.VolumeSizeLimitMB)
	}
	if cfg.Connection.MeilisearchMasterKey != "1234567890abcdef" {
		t.Fatalf("unexpected Meilisearch key %q", cfg.Connection.MeilisearchMasterKey)
	}
	if cfg.Ports.Cockpit != 19090 {
		t.Fatalf("unexpected cockpit port %d", cfg.Ports.Cockpit)
	}
	if cfg.Behavior.StartupTimeoutSec != 45 {
		t.Fatalf("unexpected startup timeout %d", cfg.Behavior.StartupTimeoutSec)
	}
	if cfg.System.PackageManager != "brew" {
		t.Fatalf("unexpected package manager %q", cfg.System.PackageManager)
	}

	text := out.String()
	for _, fragment := range []string{
		"Directory",
		"Enter a valid number.",
		"value must be at least 16 characters",
		"does not support Cockpit installation in stackctl",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("expected wizard output to contain %q:\n%s", fragment, text)
		}
	}
}

func TestRunPlainWizardWithPlatformRejectsEmptyServiceSelection(t *testing.T) {
	existingDir := t.TempDir()
	base := DefaultForStackOnPlatform("dev-stack", system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	})

	_, err := runPlainWizardWithPlatform(strings.NewReader(wizardAnswers(
		"",
		"n",
		existingDir,
		"compose.yaml",
		"n",
		"n",
		"n",
		"n",
		"n",
		"n",
	)), io.Discard, base, system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	})
	if err == nil || !strings.Contains(err.Error(), "at least one stack service must be enabled") {
		t.Fatalf("unexpected no-service error: %v", err)
	}
}

func TestPromptSessionAskHelpersCoverRetryAndEOFBranches(t *testing.T) {
	t.Run("askInt retries after invalid input", func(t *testing.T) {
		session := promptSession{
			reader: bufioReaderFor("oops\n0\n5\n"),
			out:    io.Discard,
		}

		value, err := session.askInt("Startup timeout", 30, positiveInt)
		if err != nil {
			t.Fatalf("askInt returned error: %v", err)
		}
		if value != 5 {
			t.Fatalf("unexpected parsed value %d", value)
		}
	})

	t.Run("askBool rejects invalid EOF answer", func(t *testing.T) {
		session := promptSession{
			reader: bufioReaderFor("maybe"),
			out:    io.Discard,
		}

		_, err := session.askBool("Continue", false)
		if err == nil || !strings.Contains(err.Error(), "invalid boolean answer") {
			t.Fatalf("unexpected askBool error: %v", err)
		}
	})

	t.Run("askStackDir retries until accepted", func(t *testing.T) {
		missingDir := filepath.Join(t.TempDir(), "missing")
		existingDir := t.TempDir()
		var out bytes.Buffer

		session := promptSession{
			reader: bufioReaderFor(fmt.Sprintf("%s\nn\n%s\n", missingDir, existingDir)),
			out:    &out,
		}

		dir, err := session.askStackDir(existingDir)
		if err != nil {
			t.Fatalf("askStackDir returned error: %v", err)
		}
		if dir != existingDir {
			t.Fatalf("unexpected selected dir %q", dir)
		}
		if !strings.Contains(out.String(), "does not exist yet") {
			t.Fatalf("expected askStackDir prompt to mention missing directory, got %q", out.String())
		}
	})
}

func wizardAnswers(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func bufioReaderFor(input string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(input))
}
