## stackctl snapshot restore

Restore a managed stack volume snapshot

```
stackctl snapshot restore <archive-path> [flags]
```

### Examples

```
  stackctl snapshot restore local-stack.tar --force
  stackctl snapshot restore local-stack.tar --stop --force
```

### Options

```
  -f, --force   Skip confirmation before replacing managed volumes
  -h, --help    help for restore
      --stop    Stop the running stack before replacing managed volumes
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

* [stackctl snapshot](stackctl_snapshot.md)	 - Save or restore managed service volumes

