package templates

import (
	"bytes"
	"testing"
)

func TestDevStackAssetsReturnCopies(t *testing.T) {
	tests := []struct {
		name string
		fn   func() []byte
	}{
		{name: "compose", fn: DevStackComposeYAML},
		{name: "nats", fn: DevStackNATSConfig},
		{name: "redis acl", fn: DevStackRedisACL},
		{name: "pgadmin servers", fn: DevStackPgAdminServers},
		{name: "pgpass", fn: DevStackPGPass},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := tc.fn()
			if len(first) == 0 {
				t.Fatal("expected embedded asset content")
			}

			clone := append([]byte(nil), first...)
			first[0] ^= 0xff

			second := tc.fn()
			if !bytes.Equal(second, clone) {
				t.Fatalf("expected %s accessor to return an isolated copy", tc.name)
			}
		})
	}
}
