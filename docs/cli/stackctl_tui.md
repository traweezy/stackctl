## stackctl tui

Open the interactive stack dashboard

### Synopsis

Open the interactive stack dashboard.

Use a full-screen operator view for overview, stacks, config, services,
health, and action history. The services pane includes host
ports, URLs, endpoints, DSNs, copy actions, shell handoff, and live-log
handoff in one place. The dashboard
supports manual refresh, optional auto-refresh with a saved
TUI interval, compact mode,
masked secrets by default, split inspection panes, in-TUI
config editing with diff preview, save/reset/defaults/scaffold
flows, automatic managed-stack apply on save when it is safe,
and in-TUI actions for stack lifecycle tasks. The Stacks pane
lets you inspect saved profiles, switch the active stack,
start or stop selected stack profiles, and remove profiles
without leaving the dashboard. Use
tab/shift+tab or h/l to
change sections, use j/k or [ and ] to switch the active
item inside split inspection panes, use c to copy service
values, g to jump between services or stack profiles, : or ctrl+k for the
command palette, including stack-wide connect/env/ports copy helpers,
e for a service shell, d for the Postgres db
shell, and press w from the service and health panels to open
live logs for the selected compose service in the full terminal
viewer.

```
stackctl tui [flags]
```

### Examples

```
  stackctl tui
```

### Options

```
      --debug-log-file string   Write Bubble Tea debug logs to this path
  -h, --help                    help for tui
      --mouse string            Mouse support for scrolling and click-aware navigation (auto, on, off) (default "auto")
```

### Options inherited from parent commands

```
      --accessible          Render interactive prompts and spinners in accessible mode
      --log-file string     Write internal logs to this path ('-' writes to stderr)
      --log-format string   Set the internal log format for --log-file (text, json, logfmt)
      --log-level string    Set the internal log level when --log-file is enabled
      --plain               Force the legacy plain-text config wizard instead of the form UI
  -q, --quiet               Suppress non-essential progress output
      --stack string        Select a named stack config (overrides STACKCTL_STACK and the saved current stack)
  -v, --verbose             Print extra lifecycle detail
```

### SEE ALSO

* [stackctl](stackctl.md)	 - Manage a local Podman development stack

