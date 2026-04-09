package main

import (
	"io"
	"os"
	"testing"

	"github.com/traweezy/stackctl/cmd"
	configpkg "github.com/traweezy/stackctl/internal/config"
)

func BenchmarkCLIHelp(b *testing.B) {
	benchmarkCLICommand(b, "--help")
}

func BenchmarkCLIVersion(b *testing.B) {
	benchmarkCLICommand(b, "version")
}

func BenchmarkCLITUIHelp(b *testing.B) {
	benchmarkCLICommand(b, "tui", "--help")
}

func benchmarkCLICommand(b *testing.B, args ...string) {
	b.Helper()

	originalStack, hadStack := os.LookupEnv(configpkg.StackNameEnvVar)
	if err := os.Setenv(configpkg.StackNameEnvVar, configpkg.DefaultStackName); err != nil {
		b.Fatalf("set %s: %v", configpkg.StackNameEnvVar, err)
	}
	b.Cleanup(func() {
		if !hadStack {
			_ = os.Unsetenv(configpkg.StackNameEnvVar)
			return
		}
		_ = os.Setenv(configpkg.StackNameEnvVar, originalStack)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app := cmd.NewApp()
		app.Version = version
		app.GitCommit = gitCommit
		app.BuildDate = buildDate

		root := cmd.NewRootCmd(app)
		root.SetArgs(args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)

		if err := root.Execute(); err != nil {
			b.Fatalf("execute %v: %v", args, err)
		}
	}
}
