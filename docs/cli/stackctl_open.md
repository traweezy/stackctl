## stackctl open

Open configured web UIs

### Synopsis

Open configured stack web UIs. If browser launch is unavailable, stackctl prints the URL instead.

```
stackctl open [cockpit|meilisearch|pgadmin|all] [flags]
```

### Examples

```
  stackctl open
  stackctl open cockpit
  stackctl open meilisearch
  stackctl open pgadmin
  stackctl open all
```

### Options

```
  -h, --help   help for open
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

