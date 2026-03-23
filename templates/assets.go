package templates

import _ "embed"

//go:embed dev-stack/compose.yaml
var devStackComposeYAML []byte

//go:embed dev-stack/nats.conf
var devStackNATSConfig []byte

func DevStackComposeYAML() []byte {
	return append([]byte(nil), devStackComposeYAML...)
}

func DevStackNATSConfig() []byte {
	return append([]byte(nil), devStackNATSConfig...)
}
