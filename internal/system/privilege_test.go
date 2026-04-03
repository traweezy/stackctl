package system

import (
	"reflect"
	"strings"
	"testing"
)

func TestPrivilegeCommandWithDepsUsesDirectCommandWhenAlreadyRoot(t *testing.T) {
	name, args, err := privilegeCommandWithDeps(
		func() int { return 0 },
		func(string) bool { return false },
		"systemctl",
		"enable",
		"--now",
		"cockpit.socket",
	)
	if err != nil {
		t.Fatalf("privilegeCommandWithDeps returned error: %v", err)
	}
	if name != "systemctl" {
		t.Fatalf("unexpected command name: %q", name)
	}
	if want := []string{"enable", "--now", "cockpit.socket"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: got %v want %v", args, want)
	}
}

func TestPrivilegeCommandWithDepsUsesSudoWhenNotRoot(t *testing.T) {
	name, args, err := privilegeCommandWithDeps(
		func() int { return 1000 },
		func(name string) bool { return name == "sudo" },
		"apt-get",
		"install",
		"-y",
		"podman",
	)
	if err != nil {
		t.Fatalf("privilegeCommandWithDeps returned error: %v", err)
	}
	if name != "sudo" {
		t.Fatalf("unexpected command name: %q", name)
	}
	if want := []string{"apt-get", "install", "-y", "podman"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: got %v want %v", args, want)
	}
}

func TestPrivilegeCommandWithDepsErrorsWithoutSudo(t *testing.T) {
	_, _, err := privilegeCommandWithDeps(
		func() int { return 1000 },
		func(string) bool { return false },
		"apt-get",
		"install",
		"-y",
		"podman",
	)
	if err == nil || !strings.Contains(err.Error(), "require root or passwordless sudo") {
		t.Fatalf("unexpected error: %v", err)
	}
}
