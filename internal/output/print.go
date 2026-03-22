package output

import (
	"fmt"
	"io"
)

const (
	StatusOK      = "OK"
	StatusInfo    = "INFO"
	StatusConfig  = "CONFIG"
	StatusStart   = "START"
	StatusStop    = "STOP"
	StatusRestart = "RESTART"
	StatusReset   = "RESET"
	StatusLogs    = "LOGS"
	StatusHealth  = "HEALTH"
	StatusMiss    = "MISS"
	StatusWarn    = "WARN"
	StatusFail    = "FAIL"
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
	case StatusConfig:
		return "⚙️"
	case StatusStart:
		return "🚀"
	case StatusStop:
		return "🛑"
	case StatusRestart:
		return "🔄"
	case StatusReset:
		return "🔥"
	case StatusLogs:
		return "📜"
	case StatusHealth:
		return "🩺"
	case StatusWarn:
		return "⚠️"
	case StatusMiss, StatusFail:
		return "❌"
	default:
		return "•"
	}
}
