## stackctl config edit

Edit the current config using the interactive wizard

```
stackctl config edit [flags]
```

### Examples

```
  stackctl config edit
  stackctl config edit --non-interactive
```

### Options

```
  -h, --help              help for edit
      --non-interactive   Save the current config after applying derived defaults
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

* [stackctl config](stackctl_config.md)	 - Manage persistent stack configuration

