package templates

import _ "embed"

//go:embed dev-stack/compose.yaml
var devStackComposeYAML []byte

//go:embed dev-stack/nats.conf
var devStackNATSConfig []byte

//go:embed dev-stack/redis.acl
var devStackRedisACL []byte

//go:embed dev-stack/pgadmin-servers.json
var devStackPgAdminServers []byte

//go:embed dev-stack/pgpass
var devStackPGPass []byte

func DevStackComposeYAML() []byte {
	return append([]byte(nil), devStackComposeYAML...)
}

func DevStackNATSConfig() []byte {
	return append([]byte(nil), devStackNATSConfig...)
}

func DevStackRedisACL() []byte {
	return append([]byte(nil), devStackRedisACL...)
}

func DevStackPgAdminServers() []byte {
	return append([]byte(nil), devStackPgAdminServers...)
}

func DevStackPGPass() []byte {
	return append([]byte(nil), devStackPGPass...)
}
