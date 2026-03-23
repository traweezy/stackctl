package cmd

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	configpkg "github.com/traweezy/stackctl/internal/config"
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
		Use:     "ports",
		Short:   "Show host-to-service port mappings",
		Example: "  stackctl ports",
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
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "SERVICE\tHOST\tPORTS"); err != nil {
		return err
	}
	for _, mapping := range mappings {
		if _, err := fmt.Fprintf(
			writer,
			"%s\t%s\t%s\n",
			mapping.DisplayName,
			mapping.Host,
			formatServicePort(mapping.ExternalPort, mapping.InternalPort),
		); err != nil {
			return err
		}
	}

	return writer.Flush()
}
