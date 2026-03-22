package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/traweezy/stackctl/internal/testutil"
)

func TestCLIScripts(t *testing.T) {
	binaryPath := testutil.BuildStackctlBinary(t)
	binaryDir := filepath.Dir(binaryPath)

	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("testdata", "script"),
		Setup: func(env *testscript.Env) error {
			path := binaryDir
			if currentPath := env.Getenv("PATH"); currentPath != "" {
				path += string(os.PathListSeparator) + currentPath
			}
			env.Setenv("PATH", path)
			env.Setenv("HOME", env.WorkDir)
			env.Setenv("XDG_CONFIG_HOME", filepath.Join(env.WorkDir, ".config"))
			env.Setenv("XDG_DATA_HOME", filepath.Join(env.WorkDir, ".local", "share"))
			return nil
		},
	})
}
