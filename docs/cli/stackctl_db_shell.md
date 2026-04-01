## stackctl db shell

Open psql against the configured Postgres database

```
stackctl db shell [-- <psql args...>] [flags]
```

### Examples

```
  stackctl db shell
  stackctl db shell -- -c "select version()"
  stackctl db shell -- -tAc "select current_user"
```

### Options

```
  -h, --help     help for shell
      --no-tty   Disable TTY allocation for the psql session
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

* [stackctl db](stackctl_db.md)	 - Run database-focused helpers

