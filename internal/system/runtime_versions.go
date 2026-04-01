package system

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	SupportedPodmanVersion          = "4.9.3"
	SupportedComposeProviderVersion = "1.0.6"
)

var runtimeVersionPattern = regexp.MustCompile(`(?i)\bv?([0-9]+(?:\.[0-9]+){1,2})(?:[-+][0-9A-Za-z.-]+)?\b`)

type semVersion struct {
	major int
	minor int
	patch int
}

func PodmanVersion(ctx context.Context) (string, error) {
	result, err := CaptureResult(ctx, "", "podman", "--version")
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", versionCommandError("podman --version", result)
	}

	version, ok := extractRuntimeVersion(result.Stdout + "\n" + result.Stderr)
	if !ok {
		return "", fmt.Errorf("could not determine podman version")
	}

	return version, nil
}

func PodmanComposeVersion(ctx context.Context) (string, error) {
	env := []string(nil)
	if strings.TrimSpace(os.Getenv("PODMAN_COMPOSE_PROVIDER")) == "" && CommandExists("podman-compose") {
		env = []string{"PODMAN_COMPOSE_PROVIDER=podman-compose"}
	}

	result, err := CaptureResultWithEnv(ctx, "", env, "podman", "compose", "version")
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", versionCommandError("podman compose version", result)
	}

	version, ok := extractRuntimeVersion(result.Stdout + "\n" + result.Stderr)
	if !ok {
		return "", fmt.Errorf("could not determine podman compose provider version")
	}

	return version, nil
}

func VersionAtLeast(version, minimum string) bool {
	current, ok := parseSemVersion(version)
	if !ok {
		return false
	}
	floor, ok := parseSemVersion(minimum)
	if !ok {
		return false
	}

	return current.compare(floor) >= 0
}

func extractRuntimeVersion(raw string) (string, bool) {
	match := runtimeVersionPattern.FindStringSubmatch(raw)
	if len(match) < 2 {
		return "", false
	}

	return match[1], true
}

func parseSemVersion(raw string) (semVersion, bool) {
	normalized, ok := extractRuntimeVersion(raw)
	if !ok {
		return semVersion{}, false
	}

	parts := strings.Split(normalized, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return semVersion{}, false
	}

	values := make([]int, 3)
	for idx := range parts {
		value, err := strconv.Atoi(parts[idx])
		if err != nil {
			return semVersion{}, false
		}
		values[idx] = value
	}

	return semVersion{
		major: values[0],
		minor: values[1],
		patch: values[2],
	}, true
}

func (v semVersion) compare(other semVersion) int {
	switch {
	case v.major != other.major:
		return compareInt(v.major, other.major)
	case v.minor != other.minor:
		return compareInt(v.minor, other.minor)
	default:
		return compareInt(v.patch, other.patch)
	}
}

func compareInt(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func versionCommandError(command string, result CommandResult) error {
	detail := strings.TrimSpace(result.Stderr)
	if detail == "" {
		detail = strings.TrimSpace(result.Stdout)
	}
	if detail == "" {
		detail = fmt.Sprintf("exit code %d", result.ExitCode)
	}

	return fmt.Errorf("%s failed: %s", command, detail)
}
