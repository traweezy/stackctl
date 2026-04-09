package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetBuildHelperState() {
	buildOnce = sync.Once{}
	binaryPath = ""
	buildErr = nil
}

type fatalRecorder struct {
	testing.TB
	failed  bool
	message string
}

func (r *fatalRecorder) Fatal(args ...any) {
	r.failed = true
	r.message = fmt.Sprint(args...)
}

func TestBuildStackctlBinaryFailsWhenTempDirCannotBeCreated(t *testing.T) {
	resetBuildHelperState()
	t.Cleanup(resetBuildHelperState)

	blockingPath := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blockingPath, []byte("blocking file"), 0o600); err != nil {
		t.Fatalf("write blocking path: %v", err)
	}

	t.Setenv("TMPDIR", blockingPath)
	t.Setenv("TMP", blockingPath)
	t.Setenv("TEMP", blockingPath)

	recorder := &fatalRecorder{TB: t}
	if path := BuildStackctlBinary(recorder); !recorder.failed || path != "" {
		t.Fatalf("expected BuildStackctlBinary to fail when temp dir creation breaks, path=%q failed=%v message=%q", path, recorder.failed, recorder.message)
	}
}

func TestBuildStackctlBinaryFailsWhenGoToolIsUnavailable(t *testing.T) {
	resetBuildHelperState()
	t.Cleanup(resetBuildHelperState)

	t.Setenv("PATH", t.TempDir())

	recorder := &fatalRecorder{TB: t}
	if path := BuildStackctlBinary(recorder); !recorder.failed || path == "" {
		t.Fatalf("expected BuildStackctlBinary to fail when the go tool is unavailable, path=%q failed=%v message=%q", path, recorder.failed, recorder.message)
	}
}

func TestRepoRootPanicsWhenRuntimeCallerFails(t *testing.T) {
	originalCaller := runtimeCaller
	t.Cleanup(func() { runtimeCaller = originalCaller })

	runtimeCaller = func(int) (uintptr, string, int, bool) {
		return 0, "", 0, false
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected RepoRoot to panic when runtime.Caller fails")
		}
	}()

	_ = RepoRoot()
}
