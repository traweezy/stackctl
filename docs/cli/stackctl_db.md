## stackctl db

Run database-focused helpers

### Options

```
  -h, --help   help for db
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
* [stackctl db dump](stackctl_db_dump.md)	 - Dump the configured Postgres database as SQL
* [stackctl db reset](stackctl_db_reset.md)	 - Drop and recreate the configured Postgres database
* [stackctl db restore](stackctl_db_restore.md)	 - Restore the configured Postgres database from a SQL dump
* [stackctl db shell](stackctl_db_shell.md)	 - Open psql against the configured Postgres database

