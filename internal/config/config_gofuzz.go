//go:build gofuzz

package config

import "gopkg.in/yaml.v3"

func FuzzConfigLoadAndRenderGo(data []byte) int {
	if len(data) > 1<<16 {
		return 0
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return 0
	}

	_ = cfg.normalizeSchemaVersion()
	cfg.ApplyDerivedFields()

	if _, err := Marshal(cfg); err != nil {
		return 0
	}
	_, _ = renderManagedCompose(cfg)

	return 1
}
