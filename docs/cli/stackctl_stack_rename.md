## stackctl stack rename

Rename a stack profile

```
stackctl stack rename <old-name> <new-name> [flags]
```

### Examples

```
  stackctl stack rename staging qa
  stackctl stack rename demo dev-stack
```

### Options

```
  -h, --help   help for rename
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

