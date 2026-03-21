package compose

import (
	"strings"
	"testing"
)

func TestComposeNoiseFilterSkipsProviderBanner(t *testing.T) {
	var out strings.Builder
	filter := newComposeNoiseFilter(&out)

	if _, err := filter.Write([]byte(">>>> Executing external compose provider \"/usr/bin/podman-compose\". Please see podman-compose(1) for how to disable this message. <<<<\nservice log line\n")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := filter.Flush(); err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}

	if got := out.String(); got != "service log line\n" {
		t.Fatalf("unexpected filtered output: %q", got)
	}
}

func TestComposeNoiseFilterFlushesPartialLine(t *testing.T) {
	var out strings.Builder
	filter := newComposeNoiseFilter(&out)

	if _, err := filter.Write([]byte("final partial line")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := filter.Flush(); err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}

	if got := out.String(); got != "final partial line" {
		t.Fatalf("unexpected flushed output: %q", got)
	}
}
