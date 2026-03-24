package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty/v2"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/testutil"
)

func TestTUIConfigPTYCanCreateAndScaffoldFromScratch(t *testing.T) {
	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("HOME", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	env := cliTestEnv(t, configRoot, dataRoot)

	output, err := runStackctlPTYSteps(t, binaryPath, env, []ptyStep{
		{Delay: 250 * time.Millisecond, Input: "\t\t"},
		{Delay: 150 * time.Millisecond, Input: "g"},
		{Delay: 800 * time.Millisecond, Input: "\x03"},
	}, "tui")
	if err != nil {
		t.Fatalf("tui returned error: %v\n%s", err, output)
	}

	configPath := filepath.Join(configRoot, "stackctl", "config.yaml")
	cfg, err := configpkg.Load(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if cfg.Stack.Name != "dev-stack" || !cfg.Stack.Managed {
		t.Fatalf("unexpected saved config: %+v", cfg.Stack)
	}

	composePath := filepath.Join(dataRoot, "stackctl", "stacks", "dev-stack", "compose.yaml")
	if _, err := os.Stat(composePath); err != nil {
		t.Fatalf("expected scaffolded compose file at %s: %v", composePath, err)
	}
}

func TestTUIConfigPTYCanApplyFreshConfig(t *testing.T) {
	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("HOME", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	env := cliTestEnv(t, configRoot, dataRoot)

	output, err := runStackctlPTYSteps(t, binaryPath, env, []ptyStep{
		{Delay: 250 * time.Millisecond, Input: "\t\t"},
		{Delay: 150 * time.Millisecond, Input: "A"},
		{Delay: 800 * time.Millisecond, Input: "\x03"},
	}, "tui")
	if err != nil {
		t.Fatalf("tui returned error: %v\n%s", err, output)
	}

	configPath := filepath.Join(configRoot, "stackctl", "config.yaml")
	cfg, err := configpkg.Load(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if cfg.Stack.Name != "dev-stack" || !cfg.Stack.Managed {
		t.Fatalf("unexpected saved config: %+v", cfg.Stack)
	}

	composePath := filepath.Join(dataRoot, "stackctl", "stacks", "dev-stack", "compose.yaml")
	if _, err := os.Stat(composePath); err != nil {
		t.Fatalf("expected scaffolded compose file at %s: %v", composePath, err)
	}
}

func TestTUIConfigPTYCanTypeNAndSave(t *testing.T) {
	binaryPath := testutil.BuildStackctlBinary(t)
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	t.Setenv("HOME", dataRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)
	env := cliTestEnv(t, configRoot, dataRoot)

	output, err := runStackctlPTYSteps(t, binaryPath, env, []ptyStep{
		{Delay: 250 * time.Millisecond, Input: "\t\t"},
		{Delay: 150 * time.Millisecond, Input: "\r"},
		{Delay: 150 * time.Millisecond, Input: "n"},
		{Delay: 150 * time.Millisecond, Input: "\r"},
		{Delay: 350 * time.Millisecond, Input: "\x13"},
		{Delay: 700 * time.Millisecond, Input: "\x03"},
	}, "tui")
	if err != nil {
		t.Fatalf("tui returned error: %v\n%s", err, output)
	}

	configPath := filepath.Join(configRoot, "stackctl", "config.yaml")
	cfg, err := configpkg.Load(configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if cfg.Stack.Name != "dev-stackn" {
		t.Fatalf("expected saved stack name to keep typed n, got %+v", cfg.Stack)
	}
	if !strings.Contains(cfg.Stack.Dir, "dev-stackn") {
		t.Fatalf("expected managed stack dir to track the edited name, got %+v", cfg.Stack)
	}
}

type ptyStep struct {
	Delay time.Duration
	Input string
}

func runStackctlPTYSteps(t testing.TB, binaryPath string, env []string, steps []ptyStep, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = testutil.RepoRoot()
	cmd.Env = testutil.MergeEnv(env)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("start pty command: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	go func() {
		for _, step := range steps {
			time.Sleep(step.Delay)
			_, _ = io.WriteString(ptmx, step.Input)
		}
		time.Sleep(1200 * time.Millisecond)
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
		}
	}()

	var output bytes.Buffer
	_, readErr := io.Copy(&output, ptmx)
	waitErr := cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("stackctl pty command timed out: %v", args)
	}
	if readErr != nil && !strings.Contains(readErr.Error(), "input/output error") {
		t.Fatalf("read pty output: %v", readErr)
	}
	if waitErr != nil && strings.Contains(output.String(), "program was killed: program was interrupted") {
		waitErr = nil
	}

	return output.String(), waitErr
}
