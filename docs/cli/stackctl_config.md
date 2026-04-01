## stackctl config

Manage persistent stack configuration

### Examples

```
  stackctl config view
  stackctl config validate
  stackctl config scaffold --force
```

### Options

```
  -h, --help   help for config
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
* [stackctl config edit](stackctl_config_edit.md)	 - Edit the current config using the interactive wizard
* [stackctl config init](stackctl_config_init.md)	 - Create a new stackctl config
* [stackctl config path](stackctl_config_path.md)	 - Print the resolved config path
* [stackctl config reset](stackctl_config_reset.md)	 - Reset the config to defaults or delete it
* [stackctl config scaffold](stackctl_config_scaffold.md)	 - Create or refresh the managed stack files from embedded templates
* [stackctl config validate](stackctl_config_validate.md)	 - Validate the current config
* [stackctl config view](stackctl_config_view.md)	 - Print the current config in YAML format

