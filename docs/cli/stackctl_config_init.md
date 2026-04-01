## stackctl config init

Create a new stackctl config

```
stackctl config init [flags]
```

### Examples

```
  stackctl config init
  stackctl config init --non-interactive
  stackctl config init --force
```

### Options

```
      --force             Overwrite an existing config without prompting
  -h, --help              help for init
      --non-interactive   Create the config from defaults without prompts
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

