## stackctl exec

Run a command inside a stack service

### Synopsis

Run a command inside a configured service container. Use -- before the target command so stackctl stops parsing flags.

```
stackctl exec <service> -- <command...> [flags]
```

### Examples

```
  stackctl exec postgres -- psql -U app -d app
  stackctl exec redis -- redis-cli -a secret PING
  stackctl exec seaweedfs -- weed shell
```

### Options

```
  -h, --help     help for exec
      --no-tty   Disable TTY allocation for the exec session
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

