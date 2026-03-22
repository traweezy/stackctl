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
