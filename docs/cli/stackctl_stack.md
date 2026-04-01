## stackctl stack

Manage named stack profiles

### Examples

```
  stackctl stack list
  stackctl stack current
  stackctl stack use staging
  stackctl stack clone dev-stack demo
  stackctl stack rename demo qa
  stackctl stack delete qa --purge-data --force
```

### Options

```
  -h, --help   help for stack
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
* [stackctl stack clone](stackctl_stack_clone.md)	 - Clone a stack profile into a new stack
* [stackctl stack current](stackctl_stack_current.md)	 - Print the active stack selection
* [stackctl stack delete](stackctl_stack_delete.md)	 - Delete a stack profile
* [stackctl stack list](stackctl_stack_list.md)	 - List configured stack profiles and the active selection
* [stackctl stack rename](stackctl_stack_rename.md)	 - Rename a stack profile
* [stackctl stack use](stackctl_stack_use.md)	 - Persist a stack as the default selection

