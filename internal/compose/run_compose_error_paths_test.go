package compose

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/traweezy/stackctl/internal/system"
)

func TestRunComposeReturnsRunnerErrorsBeforeFlush(t *testing.T) {
	client := Client{Runner: system.Runner{}}
	if err := client.runCompose(context.Background(), filepath.Join(t.TempDir(), "missing"), "compose", "version"); err == nil {
		t.Fatal("expected runCompose to surface runner errors")
	}
}
