## stackctl logs

Show recent stack logs or follow them live

### Synopsis

Show recent stack logs. By default this prints the last 100 lines and exits. Use --watch to keep streaming log output.

```
stackctl logs [flags]
```

### Examples

```
  stackctl logs
  stackctl logs --watch
  stackctl logs --service postgres
  stackctl logs --service meilisearch --tail 200 --watch
```

### Options

```
  -h, --help             help for logs
  -s, --service string   Filter logs to a single service (postgres|pg, redis|rd, nats, seaweedfs|seaweed, meilisearch|meili, pgadmin)
      --since string     Show logs since a relative time or timestamp
  -n, --tail int         Number of log lines to show (default 100)
  -w, --watch            Follow logs
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

