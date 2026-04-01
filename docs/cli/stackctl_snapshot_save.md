## stackctl snapshot save

Save a managed stack volume snapshot

```
stackctl snapshot save <archive-path> [flags]
```

### Examples

```
  stackctl snapshot save local-stack.tar
  stackctl snapshot save local-stack.tar --stop
```

### Options

```
  -h, --help   help for save
      --stop   Stop the running stack before exporting managed volumes
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

