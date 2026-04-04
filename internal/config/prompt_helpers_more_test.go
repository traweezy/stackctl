package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty/v2"
)

func TestClearWizardScreenWritesEscapeSequenceToTerminal(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open returned error: %v", err)
	}
	defer func() { _ = ptmx.Close() }()
	defer func() { _ = tty.Close() }()

	if err := clearWizardScreen(tty); err != nil {
		t.Fatalf("clearWizardScreen returned error: %v", err)
	}
	if err := ptmx.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline returned error: %v", err)
	}

	buf := make([]byte, 16)
	n, err := ptmx.Read(buf)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if got := string(buf[:n]); got != "\x1b[H\x1b[2J" {
		t.Fatalf("unexpected clear sequence %q", got)
	}
}

func TestClearWizardScreenSkipsAccessibleTerminals(t *testing.T) {
	t.Setenv("ACCESSIBLE", "1")

	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open returned error: %v", err)
	}
	defer func() { _ = ptmx.Close() }()
	defer func() { _ = tty.Close() }()

	if err := clearWizardScreen(tty); err != nil {
		t.Fatalf("clearWizardScreen returned error: %v", err)
	}
	if err := ptmx.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline returned error: %v", err)
	}

	buf := make([]byte, 16)
	_, err = ptmx.Read(buf)
	if err == nil {
		t.Fatal("expected accessible clearWizardScreen call to avoid terminal writes")
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) && !os.IsTimeout(err) {
		t.Fatalf("expected timeout waiting for output, got %v", err)
	}
}

func TestPromptSessionNoteAndValidationHelpers(t *testing.T) {
	var out bytes.Buffer
	session := promptSession{out: &out}

	if err := session.printNote("   "); err != nil {
		t.Fatalf("printNote returned error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected blank note to be skipped, got %q", out.String())
	}

	if err := session.printNote("Use the detected package manager."); err != nil {
		t.Fatalf("printNote returned error: %v", err)
	}
	if got := out.String(); got != "Use the detected package manager.\n" {
		t.Fatalf("unexpected note output %q", got)
	}

	if _, ok := terminalFD(os.Stdout); !ok {
		t.Fatal("expected terminalFD to accept a regular file descriptor")
	}

	if err := minLen(3)("ab"); err == nil || !strings.Contains(err.Error(), "at least 3 characters") {
		t.Fatalf("unexpected minLen error: %v", err)
	}
	if err := minLen(3)(" abc "); err != nil {
		t.Fatalf("expected trimmed minLen input to pass, got %v", err)
	}

	if err := validStackName(" staging "); err != nil {
		t.Fatalf("expected normalized stack name to pass, got %v", err)
	}
	if err := validStackName("Invalid!"); err == nil {
		t.Fatal("expected invalid stack name to fail")
	}
}

func TestConfigPathHelpersAndFileExists(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	stacksDir, err := ConfigStacksDirPath()
	if err != nil {
		t.Fatalf("ConfigStacksDirPath returned error: %v", err)
	}
	if want := filepath.Join(configRoot, "stackctl", "stacks"); stacksDir != want {
		t.Fatalf("unexpected stacks dir path: %s", stacksDir)
	}

	currentPath, err := CurrentStackPath()
	if err != nil {
		t.Fatalf("CurrentStackPath returned error: %v", err)
	}
	if want := filepath.Join(configRoot, "stackctl", CurrentStackFileName); currentPath != want {
		t.Fatalf("unexpected current stack path: %s", currentPath)
	}

	defaultPath, err := ConfigFilePathForStack(DefaultStackName)
	if err != nil {
		t.Fatalf("ConfigFilePathForStack returned error: %v", err)
	}
	if want := filepath.Join(configRoot, "stackctl", "config.yaml"); defaultPath != want {
		t.Fatalf("unexpected default config path: %s", defaultPath)
	}

	if _, err := ConfigFilePathForStack("INVALID!"); err == nil {
		t.Fatal("expected invalid stack path lookup to fail")
	}
	if err := SetCurrentStackName("INVALID!"); err == nil {
		t.Fatal("expected invalid current stack selection to fail")
	}

	filePath := filepath.Join(configRoot, "stackctl", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if fileExists(filePath) {
		t.Fatal("expected missing file to report false")
	}
	if err := os.WriteFile(filePath, []byte("stack:\n  name: dev-stack\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if !fileExists(filePath) {
		t.Fatal("expected regular file to report true")
	}
	if fileExists(filepath.Dir(filePath)) {
		t.Fatal("expected directory to report false")
	}
}

func TestSaveFailsWhenParentPathIsAFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "config-parent")
	if err := os.WriteFile(parent, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	err := Save(filepath.Join(parent, "config.yaml"), Default())
	if err == nil || !strings.Contains(err.Error(), "create config directory") {
		t.Fatalf("unexpected save error: %v", err)
	}
}

func TestSaveAndMarshalApplyDerivedManagedFields(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Stack.Name = " staging "
	cfg.Stack.Managed = true
	cfg.Stack.Dir = ""
	cfg.Stack.ComposeFile = ""

	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	wantDir, err := ManagedStackDir("staging")
	if err != nil {
		t.Fatalf("ManagedStackDir returned error: %v", err)
	}
	text := string(data)
	for _, fragment := range []string{
		"name: staging",
		"dir: " + wantDir,
		"compose_file: compose.yaml",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("marshal output missing %q:\n%s", fragment, text)
		}
	}
}

func TestManagedStackNeedsScaffoldDetectsRedisACLDrift(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	cfg.Connection.RedisACLUsername = "stackctl"
	cfg.Connection.RedisACLPassword = "redis-acl-secret"
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	cfg.Connection.RedisACLPassword = "updated-secret"

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if !needsScaffold {
		t.Fatal("expected redis ACL drift to require scaffolding")
	}
}

func TestManagedStackNeedsScaffoldDetectsPgAdminBootstrapDrift(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg := Default()
	if _, err := ScaffoldManagedStack(cfg, false); err != nil {
		t.Fatalf("ScaffoldManagedStack returned error: %v", err)
	}

	cfg.Services.PgAdmin.BootstrapServerGroup = "Staging"

	needsScaffold, err := ManagedStackNeedsScaffold(cfg)
	if err != nil {
		t.Fatalf("ManagedStackNeedsScaffold returned error: %v", err)
	}
	if !needsScaffold {
		t.Fatal("expected pgAdmin bootstrap drift to require scaffolding")
	}
}

func TestClearWizardScreenIgnoresNonFileWriters(t *testing.T) {
	if err := clearWizardScreen(io.Discard); err != nil {
		t.Fatalf("clearWizardScreen returned error: %v", err)
	}
}
