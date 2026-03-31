#!/bin/sh
set -eu

manager="${STACKCTL_PACKAGE_MANAGER:?STACKCTL_PACKAGE_MANAGER is required}"
expect_cockpit="${STACKCTL_EXPECT_COCKPIT:-0}"

assert_installed() {
  pkg="$1"

  case "$manager" in
    apt)
      dpkg-query -W -f='${Status}\n' "$pkg" | grep -q "install ok installed"
      ;;
    dnf|yum|zypper)
      rpm -q "$pkg" >/dev/null 2>&1
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

core_packages="podman podman-compose skopeo"
if [ "$manager" != "brew" ] && [ "$manager" != "apk" ]; then
  core_packages="$core_packages buildah"
fi

install_packages="$core_packages"
if [ "$expect_cockpit" = "1" ]; then
  install_packages="$install_packages cockpit cockpit-podman"
fi

echo "==> package manager: $manager"
echo "==> packages: $install_packages"

case "$manager" in
  apt)
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y $install_packages
    ;;
  dnf)
    dnf install -y $install_packages
    ;;
  yum)
    yum install -y $install_packages
    ;;
  pacman)
    pacman -Syu --noconfirm --needed $install_packages
    ;;
  zypper)
    zypper --non-interactive install $install_packages
    ;;
  apk)
    apk add $install_packages
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
