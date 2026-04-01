package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTableWritesRoundedTable(t *testing.T) {
	var buf bytes.Buffer

	err := RenderTable(&buf, []string{"Service", "Status"}, [][]string{
		{"postgres", "healthy"},
		{"redis", "ready"},
	})
	if err != nil {
		t.Fatalf("RenderTable returned error: %v", err)
	}

	output := buf.String()
	for _, fragment := range []string{"SERVICE", "STATUS", "postgres", "healthy", "redis", "ready", "╭", "╰"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("expected table output to contain %q:\n%s", fragment, output)
		}
	}
}

func TestStringsToRowCopiesValuesInOrder(t *testing.T) {
	row := stringsToRow([]string{"postgres", "5432", "healthy"})

	if len(row) != 3 {
		t.Fatalf("unexpected row length: %d", len(row))
	}
	if row[0] != "postgres" || row[1] != "5432" || row[2] != "healthy" {
		t.Fatalf("unexpected row contents: %#v", row)
	}
}
