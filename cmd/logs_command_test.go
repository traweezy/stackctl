package cmd

import (
	"strings"
	"testing"
)

func TestLogsHelpDocumentsAliasesAndWatchMode(t *testing.T) {
	stdout, _, err := executeRoot(t, "logs", "--help")
	if err != nil {
		t.Fatalf("logs --help returned error: %v", err)
	}
	if !strings.Contains(stdout, "prints the last 100 lines and exits") {
		t.Fatalf("stdout missing default logs behavior: %s", stdout)
	}
	if !strings.Contains(stdout, "postgres|pg, redis|rd, pgadmin") {
		t.Fatalf("stdout missing service aliases: %s", stdout)
	}
	if !strings.Contains(stdout, "--watch") {
		t.Fatalf("stdout missing watch flag: %s", stdout)
	}
}
