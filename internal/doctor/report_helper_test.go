package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traweezy/stackctl/internal/output"
)

func TestDefaultDependenciesListContainersCallbackUsesCaptureResult(t *testing.T) {
	dir := t.TempDir()
	writeDoctorTestScript(t, filepath.Join(dir, "podman"), `#!/bin/sh
set -eu
if [ "$1" = "ps" ]; then
  printf '[{"Id":"postgres123","Image":"postgres:latest","Names":["postgres"],"Status":"Up","State":"running","Ports":[{"host_port":5432,"container_port":5432,"protocol":"tcp"}],"CreatedAt":"now"}]'
  exit 0
fi
echo "unexpected podman args: $*" >&2
exit 1
`)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	deps := defaultDependencies()
	containers, err := deps.listContainers(context.Background())
	if err != nil {
		t.Fatalf("defaultDependencies listContainers returned error: %v", err)
	}
	if len(containers) != 1 || containers[0].Names[0] != "postgres" {
		t.Fatalf("unexpected containers: %+v", containers)
	}
}

func TestReportAddTracksCountsForAllStatuses(t *testing.T) {
	report := Report{}
	report.add(output.StatusOK, "ok")
	report.add(output.StatusWarn, "warn")
	report.add(output.StatusMiss, "miss")
	report.add(output.StatusFail, "fail")
	report.add("custom", "custom")

	if report.OKCount != 1 || report.WarnCount != 1 || report.MissCount != 1 || report.FailCount != 1 {
		t.Fatalf("unexpected report counters: %+v", report)
	}
	if len(report.Checks) != 5 {
		t.Fatalf("expected five checks, got %+v", report.Checks)
	}
	if got := report.Checks[4].Status; got != "custom" {
		t.Fatalf("unexpected custom status %q", got)
	}
}

func writeDoctorTestScript(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimPrefix(content, "\n")), 0o755); err != nil {
		t.Fatalf("write doctor test script %s: %v", path, err)
	}
}
