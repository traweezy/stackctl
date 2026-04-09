package testutil

import "testing"

func TestMergeEnvSkipsMalformedHostEnvironmentEntries(t *testing.T) {
	originalEnviron := osEnviron
	t.Cleanup(func() { osEnviron = originalEnviron })
	osEnviron = func() []string {
		return []string{
			"STACKCTL_BASE=value",
			"STACKCTL_MALFORMED",
		}
	}

	merged := MergeEnv([]string{"STACKCTL_EXTRA=extra"})
	for _, entry := range merged {
		if entry == "STACKCTL_MALFORMED" {
			t.Fatalf("expected malformed host environment entry to be skipped, got %v", merged)
		}
	}
}
