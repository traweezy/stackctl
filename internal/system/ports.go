package system

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

func PortInUse(port int) (bool, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		_ = listener.Close()
		return false, nil
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true, nil
	}

	return false, err
}

func PortListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 750*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func WaitForPort(ctx context.Context, port int, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if PortListening(port) {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for port %d: %w", port, ctx.Err())
		case <-ticker.C:
		}
	}
}
