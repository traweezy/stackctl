## stackctl services

Show full connection details for configured services

```
stackctl services [flags]
```

### Examples

```
  stackctl services
  stackctl services --json
  stackctl services --copy meilisearch-api-key
  stackctl services --copy seaweedfs
  stackctl services --copy seaweedfs-secret-key
```

### Options

```
      --copy string   Copy a service value such as postgres, meilisearch-api-key, seaweedfs, seaweedfs-secret-key, pgadmin, or cockpit
  -h, --help          help for services
  -j, --json          Print service details as JSON
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

