package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	stackctlcmd "github.com/traweezy/stackctl/cmd"
)

const (
	markdownDir    = "docs/cli"
	manDir         = "docs/man/man1"
	completionsDir = "docs/completions"
)

func main() {
	if err := generateCLIAssets(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "generate CLI assets: %v\n", err)
		os.Exit(1)
	}
}

func generateCLIAssets() error {
	rootCmd := stackctlcmd.NewRootCmd(stackctlcmd.NewApp())
	disableAutoGenTags(rootCmd)

	if err := recreateDir(markdownDir); err != nil {
		return err
	}
	if err := doc.GenMarkdownTreeCustom(rootCmd, markdownDir, func(string) string { return "" }, func(name string) string {
		return name
	}); err != nil {
		return err
	}

	if err := recreateDir(manDir); err != nil {
		return err
	}
	if err := doc.GenManTree(rootCmd, &doc.GenManHeader{
		Title:   "stackctl",
		Section: "1",
		Source:  "stackctl",
	}, manDir); err != nil {
		return err
	}
	if err := normalizeManDates(manDir); err != nil {
		return err
	}

	if err := recreateDir(completionsDir); err != nil {
		return err
	}
	if err := writeCompletionFile(filepath.Join(completionsDir, "stackctl.bash"), func(w io.Writer) error {
		return rootCmd.GenBashCompletionV2(w, true)
	}); err != nil {
		return err
	}
	if err := writeCompletionFile(filepath.Join(completionsDir, "_stackctl"), rootCmd.GenZshCompletion); err != nil {
		return err
	}
	if err := writeCompletionFile(filepath.Join(completionsDir, "stackctl.fish"), func(w io.Writer) error {
		return rootCmd.GenFishCompletion(w, true)
	}); err != nil {
		return err
	}
	if err := writeCompletionFile(filepath.Join(completionsDir, "stackctl.ps1"), rootCmd.GenPowerShellCompletionWithDesc); err != nil {
		return err
	}

	return nil
}

func disableAutoGenTags(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, child := range cmd.Commands() {
		disableAutoGenTags(child)
	}
}

func recreateDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

func writeCompletionFile(path string, generate func(io.Writer) error) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	if err := generate(file); err != nil {
		_ = file.Close()
		return err
	}

	return file.Close()
}

func normalizeManDates(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".1" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		lines := strings.Split(string(content), "\n")
		for idx, line := range lines {
			if !strings.HasPrefix(line, ".TH ") {
				continue
			}

			parts := strings.Split(line, "\"")
			if len(parts) >= 10 {
				parts[5] = ""
				lines[idx] = strings.Join(parts, "\"")
			}
			break
		}

		return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	})
}
