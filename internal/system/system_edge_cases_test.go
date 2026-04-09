package system

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestSystemEdgeCases(t *testing.T) {
	t.Run("clipboardCommand falls back to wl-copy when Wayland heuristics are unavailable", func(t *testing.T) {
		binDir := t.TempDir()
		path := filepath.Join(binDir, "wl-copy")
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		t.Setenv("WAYLAND_DISPLAY", "")
		t.Setenv("DISPLAY", "")
		t.Setenv("PATH", binDir)

		command, args, ok := clipboardCommand()
		if !ok || command != "wl-copy" || args != nil {
			t.Fatalf("unexpected clipboard fallback: command=%q args=%v ok=%v", command, args, ok)
		}
	})

	t.Run("InstallPackages covers invalid plans and empty plans", func(t *testing.T) {
		binDir := t.TempDir()
		path := filepath.Join(binDir, "apt-get")
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
		t.Setenv("PATH", binDir)

		if _, err := InstallPackages(context.Background(), Runner{}, "apt", []Requirement{"unknown"}); err == nil {
			t.Fatal("expected ResolveInstallPlan errors to surface for unknown requirements")
		}
		if packages, err := InstallPackages(context.Background(), Runner{}, "apt", nil); err != nil || packages != nil {
			t.Fatalf("expected empty requirements to noop, packages=%v err=%v", packages, err)
		}
	})

	t.Run("parseOSRelease and detectLinuxPackageManager cover fallback branches", func(t *testing.T) {
		info := parseOSRelease([]byte("BROKEN\nID=ubuntu\n"))
		if info.ID != "ubuntu" {
			t.Fatalf("expected parseOSRelease to ignore malformed lines, got %+v", info)
		}

		platform := Platform{DistroLike: []string{"mystery"}}
		if got := detectLinuxPackageManager(platform, func(name string) bool { return name == "yum" }); got != "yum" {
			t.Fatalf("expected yum fallback package manager, got %q", got)
		}
	})

	t.Run("PortInUse and parseSemVersion cover injected error paths", func(t *testing.T) {
		originalListenPort := listenPort
		t.Cleanup(func() { listenPort = originalListenPort })
		listenPort = func(string, string) (net.Listener, error) {
			return nil, errors.New("listen failed")
		}

		inUse, err := PortInUse(5432)
		if err == nil || inUse {
			t.Fatalf("expected PortInUse to return the listen failure, inUse=%v err=%v", inUse, err)
		}

		if _, ok := parseSemVersion("1"); ok {
			t.Fatal("expected short semantic versions to be rejected")
		}
		if _, ok := parseSemVersion("1.two.3"); ok {
			t.Fatal("expected non-numeric semantic versions to be rejected")
		}
	})
}
