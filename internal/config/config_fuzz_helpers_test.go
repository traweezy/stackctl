package config

import "gopkg.in/yaml.v3"

func exerciseConfigFuzzInput(data []byte) bool {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return false
	}

	_ = cfg.normalizeSchemaVersion()
	cfg.ApplyDerivedFields()

	_, _ = Marshal(cfg)
	_, _ = renderManagedCompose(cfg)
	return true
}
