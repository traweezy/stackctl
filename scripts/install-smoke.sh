#!/usr/bin/env bash
set -euo pipefail

STACKCTL_RUN_INSTALL_SMOKE=1 go test . -run '^TestInstallScriptSmokeFromLocalReleaseServer$' -count=1
