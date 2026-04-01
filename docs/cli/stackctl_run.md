## stackctl run

Run a host command with stack environment variables

### Synopsis

Run a host command with the same app-ready environment variables exposed by
`stackctl env`, optionally ensuring the selected services are running first.

```
stackctl run [service...] -- <command...> [flags]
```

### Examples

```
  stackctl run -- go test ./...
  stackctl run postgres redis -- air
  stackctl run --dry-run -- npm run dev
```

### Options

```
      --dry-run    Print the selected services, command, and injected environment without running anything
  -h, --help       help for run
      --no-start   Fail instead of starting services when they are not already ready
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

