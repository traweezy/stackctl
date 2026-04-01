## stackctl config scaffold

Create or refresh the managed stack files from embedded templates

```
stackctl config scaffold [flags]
```

### Examples

```
  stackctl config scaffold
  stackctl config scaffold --force
```

### Options

```
      --force   Overwrite managed stack files from embedded templates
  -h, --help    help for scaffold
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

