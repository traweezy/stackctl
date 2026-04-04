package system

import (
	"errors"
	"testing"
)

func TestDetectPlatformForDarwinUsesBrew(t *testing.T) {
	platform := detectPlatform("darwin", func(string) ([]byte, error) {
		t.Fatal("darwin should not read os-release")
		return nil, nil
	}, func(string) bool { return false })

	if platform.PackageManager != "brew" {
		t.Fatalf("unexpected package manager: %+v", platform)
	}
	if !platform.UsesPodmanMachine() {
		t.Fatalf("expected darwin to use podman machine: %+v", platform)
	}
	if platform.SupportsBuildah() {
		t.Fatalf("expected brew platform to skip buildah support: %+v", platform)
	}
}

func TestDetectPlatformForUbuntuUsesAptAndSystemd(t *testing.T) {
	platform := detectPlatform("linux", func(string) ([]byte, error) {
		return []byte("ID=ubuntu\nID_LIKE=debian\nVERSION_ID=24.04\nVERSION_CODENAME=noble\n"), nil
	}, func(name string) bool { return name == "systemctl" })

	if platform.PackageManager != "apt" {
		t.Fatalf("unexpected package manager: %+v", platform)
	}
	if platform.ServiceManager != ServiceManagerSystemd {
		t.Fatalf("unexpected service manager: %+v", platform)
	}
	if !platform.SupportsCockpit() {
		t.Fatalf("expected cockpit support: %+v", platform)
	}
	if platform.SupportsCockpitAutoInstall() {
		t.Fatalf("expected apt cockpit auto-install to stay disabled: %+v", platform)
	}
}

func TestDetectPlatformForFedoraPrefersDnf(t *testing.T) {
	platform := detectPlatform("linux", func(string) ([]byte, error) {
		return []byte("ID=fedora\nVERSION_ID=42\n"), nil
	}, func(name string) bool {
		return name == "dnf" || name == "systemctl"
	})

	if platform.PackageManager != "dnf" {
		t.Fatalf("unexpected package manager: %+v", platform)
	}
	if !platform.SupportsCockpitAutoInstall() {
		t.Fatalf("expected dnf cockpit auto-install support: %+v", platform)
	}
}

func TestDetectPlatformForRHELFallsBackToYum(t *testing.T) {
	platform := detectPlatform("linux", func(string) ([]byte, error) {
		return []byte("ID=rhel\nID_LIKE=\"fedora\"\n"), nil
	}, func(name string) bool {
		return name == "yum"
	})

	if platform.PackageManager != "yum" {
		t.Fatalf("unexpected package manager: %+v", platform)
	}
}

func TestDetectPlatformForAlpineUsesAPKAndOpenRC(t *testing.T) {
	platform := detectPlatform("linux", func(string) ([]byte, error) {
		return []byte("ID=alpine\nVERSION_ID=3.21\n"), nil
	}, func(name string) bool { return name == "rc-service" })

	if platform.PackageManager != "apk" {
		t.Fatalf("unexpected package manager: %+v", platform)
	}
	if platform.ServiceManager != ServiceManagerOpenRC {
		t.Fatalf("unexpected service manager: %+v", platform)
	}
	if platform.SupportsCockpit() {
		t.Fatalf("expected no cockpit support on alpine: %+v", platform)
	}
}

func TestDetectPlatformFallsBackToExecutableLookup(t *testing.T) {
	platform := detectPlatform("linux", func(string) ([]byte, error) {
		return nil, errors.New("missing")
	}, func(name string) bool {
		return name == "zypper"
	})

	if platform.PackageManager != "zypper" {
		t.Fatalf("unexpected package manager: %+v", platform)
	}
}

func TestDetectPlatformForUnsupportedGOOSUsesEmptyDefaults(t *testing.T) {
	platform := detectPlatform("freebsd", func(string) ([]byte, error) {
		t.Fatal("unsupported platforms should not read os-release")
		return nil, nil
	}, func(string) bool { return true })

	if platform.GOOS != "freebsd" {
		t.Fatalf("unexpected GOOS: %+v", platform)
	}
	if platform.PackageManager != "" || platform.ServiceManager != ServiceManagerNone {
		t.Fatalf("unsupported platform should keep empty defaults: %+v", platform)
	}
}

func TestDetectPlatformWithoutServiceManagerFallsBackToNone(t *testing.T) {
	platform := detectPlatform("linux", func(string) ([]byte, error) {
		return []byte("ID=ubuntu\nID_LIKE=debian\n"), nil
	}, func(name string) bool {
		return name == "apt-get"
	})

	if platform.ServiceManager != ServiceManagerNone {
		t.Fatalf("unexpected service manager: %+v", platform)
	}
	if platform.SupportsCockpit() {
		t.Fatalf("expected cockpit to be disabled without a service manager: %+v", platform)
	}
}

func TestParseOSReleaseHandlesQuotesAndUbuntuCodename(t *testing.T) {
	info := parseOSRelease([]byte("ID=\"ubuntu\"\nID_LIKE=\"debian test\"\nVERSION_ID=\"24.04\"\nUBUNTU_CODENAME=noble\n"))

	if info.ID != "ubuntu" {
		t.Fatalf("unexpected id: %+v", info)
	}
	if len(info.IDLike) != 2 || info.IDLike[0] != "debian" || info.IDLike[1] != "test" {
		t.Fatalf("unexpected id_like: %+v", info)
	}
	if info.VersionCodename != "noble" {
		t.Fatalf("unexpected codename: %+v", info)
	}
}

func TestDetectServiceManagerCoversFallbackBranches(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		exists   func(string) bool
		want     ServiceManager
	}{
		{
			name:     "non-linux",
			platform: Platform{GOOS: "darwin"},
			exists:   func(string) bool { return true },
			want:     ServiceManagerNone,
		},
		{
			name:     "systemd",
			platform: Platform{GOOS: "linux"},
			exists:   func(name string) bool { return name == "systemctl" },
			want:     ServiceManagerSystemd,
		},
		{
			name:     "openrc-command",
			platform: Platform{GOOS: "linux"},
			exists:   func(name string) bool { return name == "rc-service" },
			want:     ServiceManagerOpenRC,
		},
		{
			name:     "alpine-fallback",
			platform: Platform{GOOS: "linux", DistroID: "alpine"},
			exists:   func(string) bool { return false },
			want:     ServiceManagerOpenRC,
		},
		{
			name:     "unknown-linux",
			platform: Platform{GOOS: "linux", DistroID: "gentoo"},
			exists:   func(string) bool { return false },
			want:     ServiceManagerNone,
		},
	}

	for _, tc := range tests {
		if got := detectServiceManager(tc.platform, tc.exists); got != tc.want {
			t.Fatalf("%s: detectServiceManager() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDetectLinuxPackageManagerCoversKnownFamiliesAndFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		exists   func(string) bool
		want     string
	}{
		{
			name:     "arch-family",
			platform: Platform{GOOS: "linux", DistroID: "arch"},
			exists:   func(string) bool { return false },
			want:     "pacman",
		},
		{
			name:     "opensuse-family",
			platform: Platform{GOOS: "linux", DistroLike: []string{"sles"}},
			exists:   func(string) bool { return false },
			want:     "zypper",
		},
		{
			name:     "fedora-defaults-to-dnf",
			platform: Platform{GOOS: "linux", DistroID: "fedora"},
			exists:   func(string) bool { return false },
			want:     "dnf",
		},
		{
			name:     "fallback-apt-get",
			platform: Platform{GOOS: "linux", DistroID: "unknown"},
			exists:   func(name string) bool { return name == "apt-get" },
			want:     "apt",
		},
		{
			name:     "fallback-dnf",
			platform: Platform{GOOS: "linux", DistroID: "unknown"},
			exists:   func(name string) bool { return name == "dnf" },
			want:     "dnf",
		},
		{
			name:     "fallback-pacman",
			platform: Platform{GOOS: "linux", DistroID: "unknown"},
			exists:   func(name string) bool { return name == "pacman" },
			want:     "pacman",
		},
		{
			name:     "fallback-apk",
			platform: Platform{GOOS: "linux", DistroID: "unknown"},
			exists:   func(name string) bool { return name == "apk" },
			want:     "apk",
		},
		{
			name:     "fallback-brew",
			platform: Platform{GOOS: "linux", DistroID: "unknown"},
			exists:   func(name string) bool { return name == "brew" },
			want:     "brew",
		},
		{
			name:     "no-known-manager",
			platform: Platform{GOOS: "linux", DistroID: "unknown"},
			exists:   func(string) bool { return false },
			want:     "",
		},
	}

	for _, tc := range tests {
		if got := detectLinuxPackageManager(tc.platform, tc.exists); got != tc.want {
			t.Fatalf("%s: detectLinuxPackageManager() = %q, want %q", tc.name, got, tc.want)
		}
	}
}
