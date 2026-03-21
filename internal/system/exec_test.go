package system

import "testing"

func TestValidateExecutableAcceptsSupportedCommands(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"podman", "systemctl", "sysctl", "sudo", "xdg-open", "open"} {
		if err := validateExecutable(name); err != nil {
			t.Fatalf("validateExecutable(%q) returned error: %v", name, err)
		}
	}
}

func TestValidateExecutableRejectsUnexpectedCommands(t *testing.T) {
	t.Parallel()

	if err := validateExecutable("bash"); err == nil {
		t.Fatal("expected validateExecutable to reject unsupported commands")
	}
}
