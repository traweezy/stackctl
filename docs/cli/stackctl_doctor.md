## stackctl doctor

Run diagnostics and optional fixes for the local stack

```
stackctl doctor [flags]
```

### Examples

```
  stackctl doctor
  stackctl doctor --fix --yes
```

### Options

```
      --fix    Try to apply supported fixes for doctor findings
  -h, --help   help for doctor
  -y, --yes    Assume yes for automatic fix prompts
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

