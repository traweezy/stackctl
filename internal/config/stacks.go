package config

import (
	"fmt"
	"regexp"
)

var stackNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func ValidateStackName(name string) error {
	normalized := normalizeStackName(name)
	if !stackNamePattern.MatchString(normalized) {
		return fmt.Errorf("invalid stack name %q: use lowercase letters, numbers, hyphens, or underscores", normalized)
	}

	return nil
}

func defaultPostgresContainerName(stackName string) string {
	if normalizeStackName(stackName) == DefaultStackName {
		return "local-postgres"
	}
	return "stackctl-" + normalizeStackName(stackName) + "-postgres"
}

func defaultRedisContainerName(stackName string) string {
	if normalizeStackName(stackName) == DefaultStackName {
		return "local-redis"
	}
	return "stackctl-" + normalizeStackName(stackName) + "-redis"
}

func defaultNATSContainerName(stackName string) string {
	if normalizeStackName(stackName) == DefaultStackName {
		return "local-nats"
	}
	return "stackctl-" + normalizeStackName(stackName) + "-nats"
}

func defaultPgAdminContainerName(stackName string) string {
	if normalizeStackName(stackName) == DefaultStackName {
		return "local-pgadmin"
	}
	return "stackctl-" + normalizeStackName(stackName) + "-pgadmin"
}

func defaultPostgresVolumeName(stackName string) string {
	if normalizeStackName(stackName) == DefaultStackName {
		return "postgres_data"
	}
	return "stackctl-" + normalizeStackName(stackName) + "-postgres-data"
}

func defaultRedisVolumeName(stackName string) string {
	if normalizeStackName(stackName) == DefaultStackName {
		return "redis_data"
	}
	return "stackctl-" + normalizeStackName(stackName) + "-redis-data"
}

func defaultPgAdminVolumeName(stackName string) string {
	if normalizeStackName(stackName) == DefaultStackName {
		return "pgadmin_data"
	}
	return "stackctl-" + normalizeStackName(stackName) + "-pgadmin-data"
}
