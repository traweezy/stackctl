#!/bin/sh
set -eu

manager="${STACKCTL_PACKAGE_MANAGER:?STACKCTL_PACKAGE_MANAGER is required}"
expect_cockpit="${STACKCTL_EXPECT_COCKPIT:-0}"
zypper_attempts="${STACKCTL_ZYPPER_ATTEMPTS:-3}"
zypper_retry_delay_seconds="${STACKCTL_ZYPPER_RETRY_DELAY_SECONDS:-2}"

assert_installed() {
  pkg="$1"

  case "$manager" in
    apt)
      dpkg-query -W -f='${Status}\n' "$pkg" | grep -q "install ok installed"
      ;;
    dnf|yum|zypper)
      rpm -q --whatprovides "$pkg" >/dev/null 2>&1
      ;;
    pacman)
      pacman -Q "$pkg" >/dev/null 2>&1
      ;;
    apk)
      apk info -e "$pkg" >/dev/null 2>&1
      ;;
    *)
      echo "unsupported package manager for assertions: $manager" >&2
      exit 1
      ;;
  esac
}

run_zypper_install() {
  attempt=1
  while [ "$attempt" -le "$zypper_attempts" ]; do
    echo "==> zypper attempt $attempt/$zypper_attempts"
    if zypper --non-interactive clean --all >/dev/null 2>&1 || true; then
      :
    fi
    if zypper --non-interactive --gpg-auto-import-keys refresh --force &&
      zypper --non-interactive install "$@"; then
      return 0
    fi
    if [ "$attempt" -ge "$zypper_attempts" ]; then
      echo "zypper install failed after $zypper_attempts attempts" >&2
      return 1
    fi
    echo "zypper install attempt $attempt failed; cleaning metadata and retrying" >&2
    sleep "$zypper_retry_delay_seconds"
    attempt=$((attempt + 1))
  done
}

set -- podman podman-compose skopeo
if [ "$manager" != "brew" ]; then
  set -- "$@" buildah
fi
core_packages="$*"

if [ "$expect_cockpit" = "1" ]; then
  set -- "$@" cockpit cockpit-podman
fi

echo "==> package manager: $manager"
echo "==> packages: $*"

case "$manager" in
  apt)
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y "$@"
    ;;
  dnf)
    dnf install -y "$@"
    ;;
  yum)
    yum install -y "$@"
    ;;
  pacman)
    pacman -Syu --noconfirm --needed "$@"
    ;;
  zypper)
    run_zypper_install "$@"
    ;;
  apk)
    apk add "$@"
    ;;
  *)
    echo "unsupported package manager: $manager" >&2
    exit 1
    ;;
esac

for pkg in $core_packages; do
  assert_installed "$pkg"
done

if [ "$expect_cockpit" = "1" ]; then
  assert_installed cockpit
  assert_installed cockpit-podman
fi

for bin in podman podman-compose skopeo; do
  command -v "$bin"
done

if echo "$core_packages" | grep -q "buildah"; then
  command -v buildah
fi

echo "==> package-manager smoke passed for $manager"
