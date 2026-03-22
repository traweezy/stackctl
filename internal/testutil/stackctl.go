package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce  sync.Once
	binaryPath string
	buildErr   error
)

func RepoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("resolve repo root: runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func BuildStackctlBinary(t testing.TB) string {
	t.Helper()

	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "stackctl-testbin-*")
		if err != nil {
			buildErr = fmt.Errorf("create test binary dir: %w", err)
			return
		}

		binaryPath = filepath.Join(dir, "stackctl")
		// #nosec G204 -- test helper builds the local repo with the fixed
		// Go toolchain and a temp output path controlled in-process.
		cmd := exec.Command("go", "build", "-o", binaryPath, ".")
		cmd.Dir = RepoRoot()
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("build stackctl test binary: %w\n%s", err, string(output))
		}
	})

	if buildErr != nil {
		t.Fatal(buildErr)
	}

	return binaryPath
}

func MergeEnv(overrides []string) []string {
	envMap := make(map[string]string)
	order := make([]string, 0)

	for _, entry := range os.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, seen := envMap[name]; !seen {
			order = append(order, name)
		}
		envMap[name] = value
	}
	for _, entry := range overrides {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, seen := envMap[name]; !seen {
			order = append(order, name)
		}
		envMap[name] = value
	}

	merged := make([]string, 0, len(order))
	for _, name := range order {
		merged = append(merged, name+"="+envMap[name])
	}

	return merged
}
