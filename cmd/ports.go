package cmd

import (
	"context"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
)

type portMapping struct {
	Service      string
	DisplayName  string
	Host         string
	ExternalPort int
	InternalPort int
}

func newPortsCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "ports",
		Short:             "Show host-to-service port mappings",
		Example:           "  stackctl ports",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadRuntimeConfig(cmd, false)
			if err != nil {
				return err
			}

			return printPortMappings(cmd, loadPortMappings(context.Background(), cfg))
		},
	}
}

func loadPortMappings(ctx context.Context, cfg configpkg.Config) []portMapping {
	mappings := configuredPortMappings(cfg)
	services, err := runtimeServices(ctx, cfg)
	if err != nil {
		return mappings
	}

	byService := make(map[string]int, len(mappings))
	for idx, mapping := range mappings {
		byService[mapping.Service] = idx
	}

	for _, service := range services {
		idx, ok := byService[service.Name]
		if !ok {
			continue
		}

		if service.DisplayName != "" {
			mappings[idx].DisplayName = service.DisplayName
		}
		if service.Host != "" {
			mappings[idx].Host = service.Host
		}
		if service.ExternalPort > 0 {
			mappings[idx].ExternalPort = service.ExternalPort
		}
		if service.InternalPort > 0 {
			mappings[idx].InternalPort = service.InternalPort
		}
	}

	return mappings
}

func configuredPortMappings(cfg configpkg.Config) []portMapping {
	mappings := make([]portMapping, 0, len(serviceDefinitions()))
	for _, definition := range portMappingDefinitions(cfg) {
		if definition.PrimaryPort == nil {
			continue
		}
		mappings = append(mappings, portMapping{
			Service:      definition.Key,
			DisplayName:  definition.DisplayName,
			Host:         cfg.Connection.Host,
			ExternalPort: definition.PrimaryPort(cfg),
			InternalPort: definition.DefaultInternalPort,
		})
	}
	return mappings
}

func printPortMappings(cmd *cobra.Command, mappings []portMapping) error {
	rows := make([][]string, 0, len(mappings))
	for _, mapping := range mappings {
		rows = append(rows, []string{
			mapping.DisplayName,
			mapping.Host,
			formatServicePort(mapping.ExternalPort, mapping.InternalPort),
		})
	}

	return output.RenderTable(cmd.OutOrStdout(), []string{"Service", "Host", "Ports"}, rows)
}
