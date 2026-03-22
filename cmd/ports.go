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
	mappings := []portMapping{
		{
			Service:      "postgres",
			DisplayName:  "Postgres",
			Host:         cfg.Connection.Host,
			ExternalPort: cfg.Ports.Postgres,
			InternalPort: 5432,
		},
		{
			Service:      "redis",
			DisplayName:  "Redis",
			Host:         cfg.Connection.Host,
			ExternalPort: cfg.Ports.Redis,
			InternalPort: 6379,
		},
	}

	if cfg.Setup.IncludePgAdmin {
		mappings = append(mappings, portMapping{
			Service:      "pgadmin",
			DisplayName:  "pgAdmin",
			Host:         cfg.Connection.Host,
			ExternalPort: cfg.Ports.PgAdmin,
			InternalPort: 80,
		})
	}

	mappings = append(mappings, portMapping{
		Service:      "cockpit",
		DisplayName:  "Cockpit",
		Host:         cfg.Connection.Host,
		ExternalPort: cfg.Ports.Cockpit,
		InternalPort: 9090,
	})

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
