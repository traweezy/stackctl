#!/usr/bin/env bash

set -euo pipefail

log_dir="${STACKCTL_JOURNEY_SMOKE_DIR:-.tmp/journey-smoke}"
integration_regex="${STACKCTL_INTEGRATION_SMOKE_REGEX:-TestSetupNonInteractiveManagedLifecycleSmoke|TestManagedStackLifecycleSmokeFromLegacyConfig|TestManagedStackStartFailsFastWhenHostPortBusy|TestExternalStackMetadataSmoke|TestNamedStackSelectionAndPathResolution|TestNamedStackSingleRunningStackGuard}"

mkdir -p "$log_dir"

go test ./cmd \
  -run 'TestConfigInitInteractivePTYCustomizesConfig|TestTUIConfigPTYCanCreateAndScaffoldFromScratch|TestTUIOverviewPTYLaunchesWithConfig' \
  -count=1 \
  -v | tee "$log_dir/pty.log"

go test ./internal/tui \
  -run 'TestOverviewExpandedLayoutShowsPathsAndManagedMode|TestViewCanDisableAltScreenForAutomation' \
  -count=1 \
  -v | tee "$log_dir/tui-render.log"

go test ./integration \
  -tags=integration \
  -run "$integration_regex" \
  -count=1 \
  -v | tee "$log_dir/integration.log"
