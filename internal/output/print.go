package output

import (
	"fmt"
	"io"
)

const (
	StatusOK     = "OK"
	StatusInfo   = "INFO"
	StatusAction = "ACTION"
	StatusMiss   = "MISS"
	StatusWarn   = "WARN"
	StatusFail   = "FAIL"
)

func StatusLine(w io.Writer, status, message string) error {
	_, err := fmt.Fprintf(w, "%s %s\n", statusPrefix(status), message)
	return err
}

func statusPrefix(status string) string {
	switch status {
	case StatusOK:
		return "✅"
	case StatusInfo:
		return "ℹ️"
	case StatusAction:
		return "🚀"
	case StatusWarn:
		return "⚠️"
	case StatusMiss, StatusFail:
		return "❌"
	default:
		return "•"
	}
}
