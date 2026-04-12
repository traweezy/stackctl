package config

import (
	"context"
	"fmt"
	"strings"

	composecli "github.com/compose-spec/compose-go/v2/cli"
	"github.com/google/go-containerregistry/pkg/name"
)

var newComposeProjectOptions = composecli.NewProjectOptions

func validateImageReference(field, value string) *ValidationIssue {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	if _, err := name.ParseReference(trimmed, name.WeakValidation); err != nil {
		return &ValidationIssue{
			Field:   field,
			Message: fmt.Sprintf("must be a valid container image reference: %v", err),
		}
	}

	return nil
}

func validateManagedComposeFile(cfg Config) error {
	options, err := newComposeProjectOptions(
		[]string{ComposePath(cfg)},
		composecli.WithName(normalizeStackName(cfg.Stack.Name)),
	)
	if err != nil {
		return err
	}

	_, err = options.LoadProject(context.Background())
	return err
}
