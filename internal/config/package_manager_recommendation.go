package config

import (
	"strings"

	"github.com/traweezy/stackctl/internal/system"
)

var packageManagerSuggestionValues = []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "brew"}

func packageManagerWizardSuggestions(current string) []string {
	values := make([]string, 0, len(packageManagerSuggestionValues)+2)
	seen := make(map[string]struct{}, len(packageManagerSuggestionValues)+2)

	appendValue := func(value string) {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		values = append(values, normalized)
	}

	appendValue(system.CurrentPackageManagerRecommendation().Name)
	appendValue(current)
	for _, value := range packageManagerSuggestionValues {
		appendValue(value)
	}

	return values
}

func packageManagerRecommendationNote() string {
	return system.FormatPackageManagerRecommendation(system.CurrentPackageManagerRecommendation())
}
