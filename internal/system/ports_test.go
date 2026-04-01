package system

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestPortInUseDetectsBusyPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	inUse, err := PortInUse(port)
	if err != nil {
		t.Fatalf("PortInUse returned error: %v", err)
	}
	if !inUse {
		t.Fatal("expected port to be in use")
	}
}

func TestWaitForPortReturnsWhenPortStartsListening(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	port := blocker.Addr().(*net.TCPAddr).Port
	_ = blocker.Close()

	go func() {
		time.Sleep(25 * time.Millisecond)
		listener, listenErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if listenErr == nil {
			time.Sleep(50 * time.Millisecond)
			_ = listener.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := WaitForPort(ctx, port, 10*time.Millisecond); err != nil {
		t.Fatalf("WaitForPort returned error: %v", err)
	}
}

func TestPortInUseReportsFreePort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	inUse, err := PortInUse(port)
	if err != nil {
		t.Fatalf("PortInUse returned error: %v", err)
	}
	if inUse {
		t.Fatal("expected closed port to report as free")
	}
}
