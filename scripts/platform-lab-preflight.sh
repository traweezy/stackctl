#!/usr/bin/env bash

set -euo pipefail

phase="${STACKCTL_PLATFORM_LAB_PHASE:-pre-setup}"
expected_os="${STACKCTL_PLATFORM_LAB_OS:-}"
expected_arch="${STACKCTL_PLATFORM_LAB_ARCH:-}"
manager="${STACKCTL_PLATFORM_LAB_PACKAGE_MANAGER:?STACKCTL_PLATFORM_LAB_PACKAGE_MANAGER is required}"
expected_distro="${STACKCTL_PLATFORM_LAB_EXPECT_DISTRO:-}"
expect_cockpit="${STACKCTL_PLATFORM_LAB_EXPECT_COCKPIT:-0}"
require_sudo="${STACKCTL_PLATFORM_LAB_REQUIRE_SUDO:-0}"
compose_provider="${PODMAN_COMPOSE_PROVIDER:-}"
supported_podman_version="${STACKCTL_SUPPORTED_PODMAN_VERSION:-4.9.3}"
supported_compose_version="${STACKCTL_SUPPORTED_COMPOSE_VERSION:-1.0.6}"

log() {
  printf '==> %s\n' "$*"
}

warn() {
  printf 'WARN: %s\n' "$*" >&2
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

command_path() {
  command -v "$1" 2>/dev/null || true
}

require_command() {
  local name="$1"
  local hint="${2:-}"

  if ! command -v "$name" >/dev/null 2>&1; then
    if [ -n "$hint" ]; then
      fail "$hint"
    fi
    fail "required command not found: $name"
  fi
  log "$name: $(command_path "$name")"
}

extract_version() {
  local raw="$1"

  if [[ "$raw" =~ [Vv]?([0-9]+(\.[0-9]+){1,2}) ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
    return 0
  fi

  return 1
}

version_at_least() {
  local current="$1"
  local minimum="$2"
  local c1 c2 c3 m1 m2 m3

  IFS=. read -r c1 c2 c3 <<<"${current}.0.0"
  IFS=. read -r m1 m2 m3 <<<"${minimum}.0.0"

  c3="${c3:-0}"
  m3="${m3:-0}"

  if (( c1 != m1 )); then
    (( c1 > m1 ))
    return
  fi
  if (( c2 != m2 )); then
    (( c2 > m2 ))
    return
  fi

  (( c3 >= m3 ))
}

require_minimum_version() {
  local label="$1"
  local raw="$2"
  local minimum="$3"
  local version

  version="$(extract_version "$raw")" || fail "$label version could not be determined from: $raw"
  if ! version_at_least "$version" "$minimum"; then
    fail "$label $version is below supported minimum $minimum"
  fi

  log "$label version: $version (supported minimum $minimum)"
}

package_manager_command() {
  case "$1" in
    apt) echo "apt-get" ;;
    dnf) echo "dnf" ;;
    yum) echo "yum" ;;
    pacman) echo "pacman" ;;
    zypper) echo "zypper" ;;
    apk) echo "apk" ;;
    brew) echo "brew" ;;
    *) fail "unsupported platform-lab package manager: $1" ;;
  esac
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

read_linux_distro() {
  if [ ! -r /etc/os-release ]; then
    echo ""
    return
  fi

  # shellcheck disable=SC1091
  . /etc/os-release
  printf '%s %s\n' "${ID:-}" "${ID_LIKE:-}" | tr '[:upper:]' '[:lower:]'
}

assert_runner_identity() {
  local os_name arch distro_text
  os_name="$(detect_os)"
  arch="$(uname -m)"

  if [ -n "$expected_os" ] && [ "$os_name" != "$expected_os" ]; then
    fail "runner OS mismatch: expected $expected_os, got $os_name"
  fi
  if [ -n "$expected_arch" ] && [ "$arch" != "$expected_arch" ]; then
    fail "runner architecture mismatch: expected $expected_arch, got $arch"
  fi

  log "runner os: $os_name"
  log "runner arch: $arch"

  if [ "$os_name" = "linux" ] && [ -n "$expected_distro" ]; then
    distro_text="$(read_linux_distro)"
    if [ -z "$distro_text" ]; then
      fail "cannot verify Linux distro; /etc/os-release is unavailable"
    fi
    log "linux distro: $distro_text"
    case "$distro_text" in
      *"$expected_distro"*) ;;
      *)
        fail "runner distro mismatch: expected token \"$expected_distro\" in \"$distro_text\""
        ;;
    esac
  fi
}

assert_linux_privileges() {
  if [ "$(detect_os)" != "linux" ] || [ "$require_sudo" != "1" ]; then
    return
  fi

  require_command "sudo" "linux platform-lab runners must provide sudo"
  if ! sudo -n true >/dev/null 2>&1; then
    fail "linux platform-lab runners must provide passwordless sudo for package installation"
  fi
  log "passwordless sudo: available"
}

log_podman_machine_state() {
  local machine_json
  machine_json="$(podman machine list --format json 2>/dev/null || true)"
  if [ -z "$machine_json" ]; then
    fail "podman machine list failed on macOS; repair the runner Podman installation"
  fi
  log "podman machine list: $machine_json"
  if [ "$phase" = "post-setup" ]; then
    case "$machine_json" in
      "[]")
        fail "podman machine is not initialized after setup/doctor"
        ;;
    esac
    if ! printf '%s' "$machine_json" | grep -Eq '"Running":[[:space:]]*true'; then
      fail "podman machine is not running after setup/doctor"
    fi
  fi
}

assert_pre_setup() {
  local os_name pm_command
  os_name="$(detect_os)"
  pm_command="$(package_manager_command "$manager")"

  require_command "$pm_command" "self-hosted runner is missing the expected package manager command: $pm_command"
  if [ "$os_name" = "linux" ] && [ "$expect_cockpit" = "1" ]; then
    require_command "systemctl" "linux platform-lab runners that expect Cockpit must provide systemctl"
  fi

  if command -v podman >/dev/null 2>&1; then
    require_minimum_version "podman" "$(podman --version)" "$supported_podman_version"
    if podman compose version >/dev/null 2>&1; then
      require_minimum_version "podman compose provider" "$(podman compose version)" "$supported_compose_version"
    fi
    if [ "$os_name" = "darwin" ]; then
      log_podman_machine_state
    fi
  else
    warn "podman is not installed yet; stackctl setup --install is expected to install it via $manager"
  fi

  if [ -n "$compose_provider" ]; then
    log "compose provider env: $compose_provider"
    if command -v "$compose_provider" >/dev/null 2>&1; then
      log "$compose_provider: $(command_path "$compose_provider")"
    else
      warn "$compose_provider is not installed yet; stackctl setup --install is expected to install it"
    fi
  fi
}

assert_post_setup() {
  local os_name
  os_name="$(detect_os)"

  require_command "podman" "podman must be installed after setup/doctor"
  require_minimum_version "podman" "$(podman --version)" "$supported_podman_version"
  if ! podman info >/dev/null 2>&1; then
    fail "podman info failed after setup/doctor"
  fi
  log "podman runtime: ready"

  if [ -n "$compose_provider" ]; then
    require_command "$compose_provider" "expected compose provider \"$compose_provider\" is missing after setup/doctor"
  fi
  if ! podman compose version >/dev/null 2>&1; then
    fail "podman compose is not available after setup/doctor"
  fi
  require_minimum_version "podman compose provider" "$(podman compose version)" "$supported_compose_version"
  log "podman compose: ready"

  if [ "$os_name" = "darwin" ]; then
    log_podman_machine_state
  fi

  if [ "$os_name" = "linux" ] && [ "$expect_cockpit" = "1" ]; then
    local cockpit_units
    require_command "systemctl" "linux platform-lab runners that expect Cockpit must provide systemctl"
    cockpit_units="$(systemctl list-unit-files cockpit.socket --no-legend --plain 2>/dev/null || true)"
    if [ -z "$(printf '%s' "$cockpit_units" | tr -d '[:space:]')" ]; then
      fail "cockpit.socket is not installed after setup/doctor"
    fi
    if ! systemctl is-active cockpit.socket >/dev/null 2>&1; then
      fail "cockpit.socket is not active after setup/doctor"
    fi
    log "cockpit.socket: active"
  fi
}

assert_runner_identity
assert_linux_privileges

case "$phase" in
  pre-setup)
    assert_pre_setup
    ;;
  post-setup)
    assert_post_setup
    ;;
  *)
    fail "unsupported STACKCTL_PLATFORM_LAB_PHASE: $phase"
    ;;
esac

log "platform-lab preflight passed for phase: $phase"
