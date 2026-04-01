package system

import (
	"context"
	"runtime"
	"testing"
)

func TestRedisOvercommitStatus(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("expected linux host, got %s", runtime.GOOS)
	}

	status, err := RedisOvercommitStatus(context.Background())
	if err != nil {
		t.Fatalf("RedisOvercommitStatus returned error: %v", err)
	}
	if !status.Supported {
		t.Fatalf("expected linux host to report an overcommit status: %+v", status)
	}
}
