package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func BenchmarkSelectedStackServiceDefinitions(b *testing.B) {
	cfg := benchmarkCommandConfig()
	selected := benchmarkSelectedStackServices()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkCountSink = len(selectedStackServiceDefinitions(cfg, selected))
	}
}

func BenchmarkConnectionEntries(b *testing.B) {
	cfg := benchmarkCommandConfig()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkCountSink = len(connectionEntries(cfg))
	}
}

func BenchmarkFormatConnectionEntries(b *testing.B) {
	entries := connectionEntries(benchmarkCommandConfig())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkStringSink = formatConnectionEntries(entries)
	}
}

func BenchmarkFormatEnvGroupsExport(b *testing.B) {
	groups, err := envGroups(benchmarkCommandConfig(), nil)
	if err != nil {
		b.Fatalf("envGroups returned error: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkStringSink = formatEnvGroups(groups, true)
	}
}

func BenchmarkPrintEnvJSON(b *testing.B) {
	cfg := benchmarkCommandConfig()
	cmd := &cobra.Command{Use: "env"}
	var out bytes.Buffer
	cmd.SetOut(&out)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out.Reset()
		if err := printEnvJSON(cmd, cfg, nil); err != nil {
			b.Fatalf("printEnvJSON returned error: %v", err)
		}
		benchmarkCountSink = out.Len()
	}
}
