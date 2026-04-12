package config

import "testing"

func FuzzConfigLoadAndRender(f *testing.F) {
	if data, err := Marshal(Default()); err == nil {
		f.Add(data)
	}

	f.Add([]byte("schema_version: 1\nstack:\n  name: dev-stack\n"))
	f.Add([]byte("schema_version: 1\nservices:\n  postgres:\n    image: docker.io/library/postgres:16\n"))
	f.Add([]byte("stack:\n  managed: true\nsetup:\n  include_postgres: true\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			t.Skip()
		}

		exerciseConfigFuzzInput(data)
	})
}
