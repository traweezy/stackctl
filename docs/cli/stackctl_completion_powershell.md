## stackctl completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	stackctl completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
stackctl completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
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

* [stackctl completion](stackctl_completion.md)	 - Generate the autocompletion script for the specified shell

