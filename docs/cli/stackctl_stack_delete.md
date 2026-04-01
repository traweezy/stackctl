## stackctl stack delete

Delete a stack profile

### Synopsis

Delete a stack profile config. Use --purge-data to also stop and remove stackctl-managed local data for that stack.

```
stackctl stack delete <name> [flags]
```

### Examples

```
  stackctl stack delete staging
  stackctl stack delete staging --purge-data --force
```

### Options

```
      --force        Skip the confirmation prompt
  -h, --help         help for delete
      --purge-data   Stop the managed stack, remove volumes, and delete stackctl-owned local data
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

* [stackctl stack](stackctl_stack.md)	 - Manage named stack profiles

