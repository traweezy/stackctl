package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoRootResolvesProjectRoot(t *testing.T) {
	root := RepoRoot()

	if !filepath.IsAbs(root) {
		t.Fatalf("expected absolute repo root, got %q", root)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected go.mod at repo root %q: %v", root, err)
	}
}

func TestBuildStackctlBinaryBuildsOnceAndReturnsExecutable(t *testing.T) {
	first := BuildStackctlBinary(t)
	second := BuildStackctlBinary(t)

	if first != second {
		t.Fatalf("expected cached test binary path, got %q and %q", first, second)
	}

	info, err := os.Stat(first)
	if err != nil {
		t.Fatalf("stat built binary: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected executable permissions on %q", first)
	}

	cmd := exec.Command(first, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run built binary: %v\n%s", err, output)
	}
	if !strings.HasPrefix(string(output), "version: ") {
		t.Fatalf("expected version output, got %q", string(output))
	}
}

func TestMergeEnvAppliesOverridesAndIgnoresInvalidEntries(t *testing.T) {
	t.Setenv("STACKCTL_TESTUTIL_BASE", "base")

	merged := MergeEnv([]string{
		"STACKCTL_TESTUTIL_BASE=override",
		"STACKCTL_TESTUTIL_EXTRA=extra",
		"STACKCTL_TESTUTIL_INVALID",
	})

	envMap := make(map[string]string, len(merged))
	for _, entry := range merged {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("expected NAME=value entry, got %q", entry)
		}
		envMap[name] = value
	}

	if envMap["STACKCTL_TESTUTIL_BASE"] != "override" {
		t.Fatalf("expected override to win, got %q", envMap["STACKCTL_TESTUTIL_BASE"])
	}
	if envMap["STACKCTL_TESTUTIL_EXTRA"] != "extra" {
		t.Fatalf("expected extra entry to be added, got %q", envMap["STACKCTL_TESTUTIL_EXTRA"])
	}
	if _, ok := envMap["STACKCTL_TESTUTIL_INVALID"]; ok {
		t.Fatal("expected invalid override entry without '=' to be ignored")
	}

	baseIndex := -1
	extraIndex := -1
	for idx, entry := range merged {
		switch {
		case strings.HasPrefix(entry, "STACKCTL_TESTUTIL_BASE="):
			baseIndex = idx
		case strings.HasPrefix(entry, "STACKCTL_TESTUTIL_EXTRA="):
			extraIndex = idx
		}
	}
	if baseIndex == -1 || extraIndex == -1 {
		t.Fatalf("expected merged env to contain both test entries, got %v", merged)
	}
	if extraIndex < baseIndex {
		t.Fatalf("expected new override key to be appended after existing env key, got base=%d extra=%d", baseIndex, extraIndex)
	}
}
