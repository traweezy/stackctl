package output

import (
	"bytes"
	"testing"
)

func TestStatusLineFormatsOutput(t *testing.T) {
	var buf bytes.Buffer

	if err := StatusLine(&buf, StatusOK, "config file found"); err != nil {
		t.Fatalf("StatusLine returned error: %v", err)
	}

	if buf.String() != "✅ config file found\n" {
		t.Fatalf("unexpected output: %q", buf.String())
	}
}

func TestStatusLineUsesCommandSpecificEmojis(t *testing.T) {
	cases := []struct {
		status  string
		message string
		want    string
	}{
		{status: StatusStart, message: "starting stack", want: "🚀 starting stack\n"},
		{status: StatusStop, message: "stopping stack", want: "🛑 stopping stack\n"},
		{status: StatusRestart, message: "restarting stack", want: "🔄 restarting stack\n"},
		{status: StatusReset, message: "resetting stack", want: "🔥 resetting stack\n"},
		{status: StatusConfig, message: "config updated", want: "⚙️ config updated\n"},
	}

	for _, tc := range cases {
		var buf bytes.Buffer
		if err := StatusLine(&buf, tc.status, tc.message); err != nil {
			t.Fatalf("StatusLine returned error: %v", err)
		}
		if buf.String() != tc.want {
			t.Fatalf("unexpected output for %s: %q", tc.status, buf.String())
		}
	}
}

func TestStatusLineCoversRemainingStatusesAndDefault(t *testing.T) {
	cases := []struct {
		status  string
		message string
		want    string
	}{
		{status: StatusInfo, message: "info", want: "ℹ️ info\n"},
		{status: StatusLogs, message: "logs", want: "📜 logs\n"},
		{status: StatusHealth, message: "health", want: "🩺 health\n"},
		{status: StatusWarn, message: "warn", want: "⚠️ warn\n"},
		{status: StatusMiss, message: "miss", want: "❌ miss\n"},
		{status: StatusFail, message: "fail", want: "❌ fail\n"},
		{status: "UNKNOWN", message: "default", want: "• default\n"},
	}

	for _, tc := range cases {
		var buf bytes.Buffer
		if err := StatusLine(&buf, tc.status, tc.message); err != nil {
			t.Fatalf("StatusLine returned error: %v", err)
		}
		if buf.String() != tc.want {
			t.Fatalf("unexpected output for %s: %q", tc.status, buf.String())
		}
	}
}
