package system

import (
	"context"
	"testing"
)

func TestListContainersParsesPodmanJSON(t *testing.T) {
	containers, err := ListContainers(context.Background(), func(context.Context, string, string, ...string) (CommandResult, error) {
		return CommandResult{
			Stdout: `[{"Names":["local-postgres"],"State":"running","Ports":[{"host_port":5432,"container_port":5432,"protocol":"tcp"}]}]`,
		}, nil
	})
	if err != nil {
		t.Fatalf("ListContainers returned error: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("unexpected container count: %d", len(containers))
	}
	if containers[0].Ports[0].HostPort != 5432 {
		t.Fatalf("unexpected host port: %+v", containers[0].Ports)
	}
}

func TestFilterContainersByNameMatchesConfiguredNames(t *testing.T) {
	filtered := FilterContainersByName([]Container{
		{Names: []string{"local-postgres"}},
		{Names: []string{"local-redis"}},
		{Names: []string{"unrelated"}},
	}, []string{"local-postgres", "local-redis"})

	if len(filtered) != 2 {
		t.Fatalf("unexpected filtered containers: %+v", filtered)
	}
}
