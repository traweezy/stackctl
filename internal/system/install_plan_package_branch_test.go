package system

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInstallPlanAdditionalBranches(t *testing.T) {
	_, err := ResolveInstallPlan("pkgsrc", []Requirement{RequirementPodman})
	if err == nil || !strings.Contains(err.Error(), "unsupported package manager") {
		t.Fatalf("unexpected unsupported package manager error: %v", err)
	}

	plan, err := ResolveInstallPlan("yum", []Requirement{
		RequirementPodman,
		RequirementPodman,
		RequirementComposeProvider,
		RequirementComposeProvider,
	})
	if err != nil {
		t.Fatalf("ResolveInstallPlan returned error: %v", err)
	}
	if got, want := strings.Join(plan.Packages, ","), "podman,podman-compose"; got != want {
		t.Fatalf("unexpected deduplicated packages: got %q want %q", got, want)
	}
}

func TestInstallPackagesCoversDNFYumAndAPKBackends(t *testing.T) {
	originalEUID := currentEUID
	currentEUID = func() int { return 0 }
	t.Cleanup(func() { currentEUID = originalEUID })

	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")

	writeScript := func(name string) {
		t.Helper()
		path := filepath.Join(dir, name)
		body := "#!/bin/sh\nprintf '" + name + " %s\\n' \"$*\" >> \"" + logPath + "\"\n"
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", name, err)
		}
	}

	for _, name := range []string{"dnf", "yum", "apk"} {
		writeScript(name)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	runner := Runner{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	if _, err := InstallPackages(context.Background(), runner, "dnf", []Requirement{RequirementPodman}); err != nil {
		t.Fatalf("dnf install returned error: %v", err)
	}
	if _, err := InstallPackages(context.Background(), runner, "yum", []Requirement{RequirementPodman}); err != nil {
		t.Fatalf("yum install returned error: %v", err)
	}
	if _, err := InstallPackages(context.Background(), runner, "apk", []Requirement{RequirementPodman}); err != nil {
		t.Fatalf("apk install returned error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	output := string(data)
	for _, fragment := range []string{
		"dnf install -y podman",
		"yum install -y podman",
		"apk add podman",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("command log missing %q:\n%s", fragment, output)
		}
	}
}

func TestRunZypperInstallWithRetryAdditionalBranches(t *testing.T) {
	originalEUID := currentEUID
	currentEUID = func() int { return 0 }
	t.Cleanup(func() { currentEUID = originalEUID })

	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	path := filepath.Join(dir, "zypper")
	script := "#!/bin/sh\nprintf 'zypper %s\\n' \"$*\" >> \"" + logPath + "\"\nexit 1\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write zypper stub: %v", err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{Stdout: &stdout, Stderr: &stderr}

	err := runZypperInstallWithRetry(ctx, runner, []string{"podman"})
	if err == nil || err != context.Canceled {
		t.Fatalf("expected canceled zypper install, got %v", err)
	}

	err = runZypperInstallWithRetry(context.Background(), runner, []string{"podman"})
	if err == nil || !strings.Contains(err.Error(), "zypper install failed after 3 attempts") {
		t.Fatalf("expected zypper retry exhaustion error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "zypper install attempt 3/3") {
		t.Fatalf("expected stdout to include the final attempt notice:\n%s", stdout.String())
	}
}
