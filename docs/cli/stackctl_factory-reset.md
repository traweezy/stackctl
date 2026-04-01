## stackctl factory-reset

DANGEROUS: delete all stackctl local config and data

### Synopsis

DANGEROUS: stop managed stacks discovered under stackctl's local data directory, remove their volumes, and then delete all stackctl-owned config and data directories.

```
stackctl factory-reset [flags]
```

### Examples

```
  stackctl factory-reset
  stackctl factory-reset --force
```

### Options

```
  -f, --force   Skip the DANGEROUS confirmation prompt
  -h, --help    help for factory-reset
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

