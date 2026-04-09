package main

import (
	"fmt"
	"io"
	"io/fs"
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

var (
	openCLIAssetsRoot          = os.OpenRoot
	closeCLIAssetsRoot         = func(root *os.Root) error { return root.Close() }
	newCLIAssetsRootCommand    = func() *cobra.Command { return stackctlcmd.NewRootCmd(stackctlcmd.NewApp()) }
	recreateCLIAssetsDir       = recreateDir
	generateCLIAssetsMarkdown  = doc.GenMarkdownTreeCustom
	generateCLIAssetsMan       = doc.GenManTree
	normalizeCLIAssetsManDates = normalizeManDates
	writeCLIAssetsCompletion   = writeCompletionFile
)

func main() {
	if err := generateCLIAssets(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "generate CLI assets: %v\n", err)
		os.Exit(1)
	}
}

func generateCLIAssets() (err error) {
	repoRoot, err := openCLIAssetsRoot(".")
	if err != nil {
		return err
	}
	defer func() {
		closeErr := closeCLIAssetsRoot(repoRoot)
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	rootCmd := newCLIAssetsRootCommand()
	disableAutoGenTags(rootCmd)

	if err := recreateCLIAssetsDir(repoRoot, markdownDir); err != nil {
		return err
	}
	if err := generateCLIAssetsMarkdown(rootCmd, markdownDir, func(string) string { return "" }, func(name string) string {
		return name
	}); err != nil {
		return err
	}

	if err := recreateCLIAssetsDir(repoRoot, manDir); err != nil {
		return err
	}
	if err := generateCLIAssetsMan(rootCmd, &doc.GenManHeader{
		Title:   "stackctl",
		Section: "1",
		Source:  "stackctl",
	}, manDir); err != nil {
		return err
	}
	if err := normalizeCLIAssetsManDates(repoRoot, manDir); err != nil {
		return err
	}

	if err := recreateCLIAssetsDir(repoRoot, completionsDir); err != nil {
		return err
	}
	if err := writeCLIAssetsCompletion(repoRoot, filepath.Join(completionsDir, "stackctl.bash"), func(w io.Writer) error {
		return rootCmd.GenBashCompletionV2(w, true)
	}); err != nil {
		return err
	}
	if err := writeCLIAssetsCompletion(repoRoot, filepath.Join(completionsDir, "_stackctl"), rootCmd.GenZshCompletion); err != nil {
		return err
	}
	if err := writeCLIAssetsCompletion(repoRoot, filepath.Join(completionsDir, "stackctl.fish"), func(w io.Writer) error {
		return rootCmd.GenFishCompletion(w, true)
	}); err != nil {
		return err
	}
	if err := writeCLIAssetsCompletion(repoRoot, filepath.Join(completionsDir, "stackctl.ps1"), rootCmd.GenPowerShellCompletionWithDesc); err != nil {
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

func recreateDir(root *os.Root, dir string) error {
	if err := root.RemoveAll(dir); err != nil {
		return err
	}
	return root.MkdirAll(dir, 0o750)
}

func writeCompletionFile(root *os.Root, path string, generate func(io.Writer) error) error {
	file, err := root.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	if err := generate(file); err != nil {
		_ = file.Close()
		return err
	}

	return file.Close()
}

func normalizeManDates(root *os.Root, path string) error {
	return fs.WalkDir(root.FS(), path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".1" {
			return nil
		}

		content, err := root.ReadFile(path)
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

		return root.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
	})
}
