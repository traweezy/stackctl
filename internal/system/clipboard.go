package system

import (
	"context"
	"fmt"
	"strings"
)

func CopyToClipboard(ctx context.Context, runner Runner, value string) error {
	command, args, ok := clipboardCommand()
	if !ok {
		return fmt.Errorf("no supported clipboard command found; install wl-copy, xclip, or xsel")
	}

	runner.Stdin = strings.NewReader(value)
	return runner.Run(ctx, "", command, args...)
}

func clipboardCommand() (string, []string, bool) {
	switch {
	case CommandExists("wl-copy"):
		return "wl-copy", nil, true
	case CommandExists("xclip"):
		return "xclip", []string{"-selection", "clipboard"}, true
	case CommandExists("xsel"):
		return "xsel", []string{"--clipboard", "--input"}, true
	case CommandExists("pbcopy"):
		return "pbcopy", nil, true
	default:
		return "", nil, false
	}
}
