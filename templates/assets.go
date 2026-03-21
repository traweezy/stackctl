package templates

import _ "embed"

//go:embed dev-stack/compose.yaml
var devStackComposeYAML []byte

func DevStackComposeYAML() []byte {
	return append([]byte(nil), devStackComposeYAML...)
}
