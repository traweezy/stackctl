## stackctl

Manage a local Podman development stack

### Examples

```
  stackctl setup
  stackctl start
  stackctl --stack staging start
  stackctl stack list
  stackctl tui
  stackctl services
  stackctl exec postgres -- psql -U app -d app
```

### Options

```
      --accessible          Render interactive prompts and spinners in accessible mode
  -h, --help                help for stackctl
      --log-file string     Write internal logs to this path ('-' writes to stderr)
      --log-format string   Set the internal log format for --log-file (text, json, logfmt)
      --log-level string    Set the internal log level when --log-file is enabled
      --plain               Force the legacy plain-text config wizard instead of the form UI
  -q, --quiet               Suppress non-essential progress output
      --stack string        Select a named stack config (overrides STACKCTL_STACK and the saved current stack)
  -v, --verbose             Print extra lifecycle detail
```

### SEE ALSO

* [stackctl completion](stackctl_completion.md)	 - Generate the autocompletion script for the specified shell
* [stackctl config](stackctl_config.md)	 - Manage persistent stack configuration
* [stackctl connect](stackctl_connect.md)	 - Print minimal connection strings and URLs
* [stackctl db](stackctl_db.md)	 - Run database-focused helpers
* [stackctl doctor](stackctl_doctor.md)	 - Run diagnostics and optional fixes for the local stack
* [stackctl env](stackctl_env.md)	 - Print app-ready environment variables
* [stackctl exec](stackctl_exec.md)	 - Run a command inside a stack service
* [stackctl factory-reset](stackctl_factory-reset.md)	 - DANGEROUS: delete all stackctl local config and data
* [stackctl health](stackctl_health.md)	 - Check whether the local stack is reachable
* [stackctl logs](stackctl_logs.md)	 - Show recent stack logs or follow them live
* [stackctl open](stackctl_open.md)	 - Open configured web UIs
* [stackctl ports](stackctl_ports.md)	 - Show host-to-service port mappings
* [stackctl reset](stackctl_reset.md)	 - Bring the stack down and optionally wipe volumes
* [stackctl restart](stackctl_restart.md)	 - Restart the local development stack or selected services
* [stackctl run](stackctl_run.md)	 - Run a host command with stack environment variables
* [stackctl services](stackctl_services.md)	 - Show full connection details for configured services
* [stackctl setup](stackctl_setup.md)	 - Prepare the local machine and stackctl config
* [stackctl snapshot](stackctl_snapshot.md)	 - Save or restore managed service volumes
* [stackctl stack](stackctl_stack.md)	 - Manage named stack profiles
* [stackctl start](stackctl_start.md)	 - Start the local development stack or selected services
* [stackctl status](stackctl_status.md)	 - Show status for containers in this stack
* [stackctl stop](stackctl_stop.md)	 - Stop the local development stack or selected services
* [stackctl tui](stackctl_tui.md)	 - Open the interactive stack dashboard
* [stackctl version](stackctl_version.md)	 - Print version information

