## stackctl config reset

Reset the config to defaults or delete it

```
stackctl config reset [flags]
```

### Examples

```
  stackctl config reset
  stackctl config reset --delete --force
```

### Options

```
      --delete   Delete the config file instead of resetting it
      --force    Skip confirmation
  -h, --help     help for reset
      --yes      Assume yes for confirmation prompts
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

