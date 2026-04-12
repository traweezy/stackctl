package cmd

import "testing"

func TestFormatConnectionEntriesHandlesEmptyInput(t *testing.T) {
	if got := formatConnectionEntries(nil); got != "" {
		t.Fatalf("expected empty connection entry output, got %q", got)
	}
}

func TestFormatEnvGroupsHandlesSkippedGroupsAndNonExportMode(t *testing.T) {
	groups := []envGroup{
		{Title: "skip"},
		{
			Title: "Postgres",
			Entries: []envEntry{
				{Name: "DATABASE_URL", Value: "postgres://app"},
			},
		},
	}

	got := formatEnvGroups(groups, false)
	want := "# Postgres\nDATABASE_URL='postgres://app'"
	if got != want {
		t.Fatalf("unexpected env group rendering:\nwant: %q\ngot:  %q", want, got)
	}
}
