## stackctl env

Print app-ready environment variables

### Synopsis

Print app-ready environment variables from the current stack config.

By default this prints shell-safe KEY=value assignments. Use --export
when you want export-prefixed lines for eval/source workflows, or --json
for tooling.

```
stackctl env [service...] [flags]
```

### Examples

```
  stackctl env
  stackctl env --export
  stackctl env postgres redis
  stackctl env --json
```

### Options

```
      --export   Prefix assignments with export for shell eval workflows
  -h, --help     help for env
  -j, --json     Print environment variables as JSON
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

