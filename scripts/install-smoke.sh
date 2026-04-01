#!/usr/bin/env bash
set -euo pipefail

STACKCTL_RUN_INSTALL_SMOKE=1 go test . -run '^TestInstallScript(SmokeFromLocalReleaseServer|InstallsRequestedVersionFromLocalReleaseServer)$' -count=1
