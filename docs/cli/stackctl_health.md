## stackctl health

Check whether the local stack is reachable

```
stackctl health [flags]
```

### Examples

```
  stackctl health
  stackctl health --watch --interval 2
```

### Options

```
  -h, --help           help for health
  -i, --interval int   Watch interval in seconds (default 5)
  -w, --watch          Continuously rerun health checks
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

