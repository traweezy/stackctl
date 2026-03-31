package system

import (
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

type ServiceManager string

const (
	ServiceManagerNone    ServiceManager = "none"
	ServiceManagerOpenRC  ServiceManager = "openrc"
	ServiceManagerSystemd ServiceManager = "systemd"
)

type Platform struct {
	GOOS            string
	DistroID        string
	DistroLike      []string
	VersionID       string
	VersionCodename string
	PackageManager  string
	ServiceManager  ServiceManager
}

type osReleaseInfo struct {
	ID              string
	IDLike          []string
	VersionID       string
	VersionCodename string
}

func CurrentPlatform() Platform {
	return detectPlatform(runtime.GOOS, os.ReadFile, CommandExists)
}

func DetectPackageManager() string {
	return CurrentPlatform().PackageManager
}

func (p Platform) SupportsBuildah() bool {
	return normalizePackageManager(p.PackageManager) != "brew"
}

func (p Platform) SupportsCockpit() bool {
	switch normalizePackageManager(p.PackageManager) {
	case "apt", "dnf", "yum", "pacman", "zypper":
		return p.ServiceManager == ServiceManagerSystemd
	default:
		return false
	}
}

func (p Platform) SupportsCockpitAutoInstall() bool {
	switch normalizePackageManager(p.PackageManager) {
	case "dnf", "yum", "pacman", "zypper":
		return p.ServiceManager == ServiceManagerSystemd
	default:
		return false
	}
}

func (p Platform) SupportsCockpitAutoEnable() bool {
	return p.SupportsCockpit() && p.ServiceManager == ServiceManagerSystemd
}

func (p Platform) SupportsSSCheck() bool {
	return p.GOOS == "linux"
}

func (p Platform) UsesPodmanMachine() bool {
	return p.GOOS == "darwin"
}

func detectPlatform(goos string, readFile func(string) ([]byte, error), commandExists func(string) bool) Platform {
	platform := Platform{
		GOOS:           goos,
		ServiceManager: ServiceManagerNone,
	}

	switch goos {
	case "darwin":
		platform.PackageManager = "brew"
		return platform
	case "linux":
		info, err := loadOSRelease(readFile)
		if err == nil {
			platform.DistroID = info.ID
			platform.DistroLike = append([]string(nil), info.IDLike...)
			platform.VersionID = info.VersionID
			platform.VersionCodename = info.VersionCodename
		}
		platform.ServiceManager = detectServiceManager(platform, commandExists)
		platform.PackageManager = detectLinuxPackageManager(platform, commandExists)
		return platform
	default:
		return platform
	}
}

func loadOSRelease(readFile func(string) ([]byte, error)) (osReleaseInfo, error) {
	paths := []string{"/etc/os-release", "/usr/lib/os-release"}
	for _, path := range paths {
		data, err := readFile(path)
		if err != nil {
			continue
		}
		return parseOSRelease(data), nil
	}

	return osReleaseInfo{}, os.ErrNotExist
}

func parseOSRelease(data []byte) osReleaseInfo {
	values := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, `"'`)
		}
		values[key] = value
	}

	codename := strings.TrimSpace(values["VERSION_CODENAME"])
	if codename == "" {
		codename = strings.TrimSpace(values["UBUNTU_CODENAME"])
	}

	like := strings.Fields(strings.TrimSpace(values["ID_LIKE"]))
	for idx, entry := range like {
		like[idx] = strings.ToLower(strings.TrimSpace(entry))
	}

	return osReleaseInfo{
		ID:              strings.ToLower(strings.TrimSpace(values["ID"])),
		IDLike:          like,
		VersionID:       strings.TrimSpace(values["VERSION_ID"]),
		VersionCodename: codename,
	}
}

func detectServiceManager(platform Platform, commandExists func(string) bool) ServiceManager {
	if platform.GOOS != "linux" {
		return ServiceManagerNone
	}
	if commandExists("systemctl") {
		return ServiceManagerSystemd
	}
	if commandExists("rc-service") {
		return ServiceManagerOpenRC
	}
	if platform.matchesDistro("alpine") {
		return ServiceManagerOpenRC
	}

	return ServiceManagerNone
}

func detectLinuxPackageManager(platform Platform, commandExists func(string) bool) string {
	switch {
	case platform.matchesDistro("ubuntu", "debian", "linuxmint", "pop", "neon", "elementary", "zorin"):
		return "apt"
	case platform.matchesDistro("fedora", "centos", "rhel", "rocky", "almalinux", "ol", "amzn"):
		if commandExists("dnf") {
			return "dnf"
		}
		if commandExists("yum") {
			return "yum"
		}
		return "dnf"
	case platform.matchesDistro("arch", "manjaro", "endeavouros"):
		return "pacman"
	case platform.matchesDistro("opensuse", "opensuse-leap", "opensuse-tumbleweed", "sles", "sle_hpc"):
		return "zypper"
	case platform.matchesDistro("alpine"):
		return "apk"
	}

	switch {
	case commandExists("apt-get"):
		return "apt"
	case commandExists("dnf"):
		return "dnf"
	case commandExists("yum"):
		return "yum"
	case commandExists("pacman"):
		return "pacman"
	case commandExists("zypper"):
		return "zypper"
	case commandExists("apk"):
		return "apk"
	case commandExists("brew"):
		return "brew"
	default:
		return ""
	}
}

func (p Platform) matchesDistro(ids ...string) bool {
	targets := make([]string, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, strings.ToLower(strings.TrimSpace(id)))
	}
	if slices.Contains(targets, strings.ToLower(strings.TrimSpace(p.DistroID))) {
		return true
	}
	for _, like := range p.DistroLike {
		if slices.Contains(targets, like) {
			return true
		}
	}
	return false
}

func normalizePackageManager(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
