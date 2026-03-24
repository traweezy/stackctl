package output

import (
	"io"

	prettytable "github.com/jedib0t/go-pretty/v6/table"
)

func RenderTable(w io.Writer, header []string, rows [][]string) error {
	table := prettytable.NewWriter()
	table.SetOutputMirror(w)
	table.SetStyle(prettytable.StyleRounded)
	table.AppendHeader(stringsToRow(header))
	for _, row := range rows {
		table.AppendRow(stringsToRow(row))
	}
	table.Render()
	return nil
}

func stringsToRow(values []string) prettytable.Row {
	row := make(prettytable.Row, 0, len(values))
	for _, value := range values {
		row = append(row, value)
	}
	return row
}
