package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/system"
)

type Client struct {
	Runner system.Runner
}

type CaptureFunc func(context.Context, string, string, ...string) (system.CommandResult, error)

type composeNames []string

func (n *composeNames) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		*n = nil
		return nil
	}

	if len(trimmed) > 0 && trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return err
		}
		*n = composeNames{value}
		return nil
	}

	var values []string
	if err := json.Unmarshal(trimmed, &values); err != nil {
		return err
	}
	*n = composeNames(values)
	return nil
}

type composePublisher struct {
	PublishedPort int    `json:"PublishedPort"`
	TargetPort    int    `json:"TargetPort"`
	Protocol      string `json:"Protocol"`
}

type composePorts []system.ContainerPort

func (p *composePorts) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		*p = nil
		return nil
	}

	if len(trimmed) > 0 && trimmed[0] == '"' {
		*p = nil
		return nil
	}

	var ports []system.ContainerPort
	if err := json.Unmarshal(trimmed, &ports); err != nil {
		return err
	}
	*p = composePorts(ports)
	return nil
}

type composeContainer struct {
	ID         string             `json:"ID"`
	LegacyID   string             `json:"Id"`
	Image      string             `json:"Image"`
	Name       string             `json:"Name"`
	Names      composeNames       `json:"Names"`
	Status     string             `json:"Status"`
	State      string             `json:"State"`
	Ports      composePorts       `json:"Ports"`
	Publishers []composePublisher `json:"Publishers"`
	CreatedAt  string             `json:"CreatedAt"`
}

func (c Client) Up(ctx context.Context, cfg configpkg.Config) error {
	return c.runComposeQuiet(ctx, cfg.Stack.Dir, composeArgs(cfg, "up", "-d")...)
}

func (c Client) Down(ctx context.Context, cfg configpkg.Config, removeVolumes bool) error {
	args := composeArgs(cfg, "down")
	if removeVolumes {
		args = append(args, "-v")
	}

	return c.runComposeQuiet(ctx, cfg.Stack.Dir, args...)
}

func (c Client) Logs(ctx context.Context, cfg configpkg.Config, tail int, follow bool, since, service string) error {
	args := composeArgs(cfg, "logs")
	if follow {
		args = append(args, "-f")
	}
	args = append(args, "--tail", strconv.Itoa(tail))
	if since != "" {
		args = append(args, "--since", since)
	}
	if service != "" {
		args = append(args, service)
	}

	return c.runCompose(ctx, cfg.Stack.Dir, args...)
}

func (c Client) ContainerLogs(ctx context.Context, containerName string, tail int, follow bool, since string) error {
	args := []string{
		"logs",
	}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, "--tail", strconv.Itoa(tail))
	if since != "" {
		args = append(args, "--since", since)
	}
	args = append(args, containerName)

	return c.Runner.Run(ctx, "", "podman", args...)
}

func ListContainers(ctx context.Context, dir, composePath string, capture CaptureFunc) ([]system.Container, error) {
	result, err := capture(ctx, dir, "podman", composeArgsForPath(composePath, "ps", "--format", "json")...)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(cleanComposeOutput(result.Stderr))
		if detail == "" {
			detail = strings.TrimSpace(cleanComposeOutput(result.Stdout))
		}
		if detail == "" {
			detail = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		return nil, fmt.Errorf("podman compose ps failed: %s", detail)
	}

	return parseComposeContainerOutput(result.Stdout)
}

func (c Client) runCompose(ctx context.Context, dir string, args ...string) error {
	runner, flush := filteredRunner(c.Runner)
	err := runner.Run(ctx, dir, "podman", args...)
	flushErr := flush()
	if err != nil {
		return err
	}

	return flushErr
}

func (c Client) runComposeQuiet(ctx context.Context, dir string, args ...string) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	runner, flush := filteredRunner(system.Runner{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	err := runner.Run(ctx, dir, "podman", args...)
	flushErr := flush()
	if err != nil || flushErr != nil {
		if c.Runner.Stdout != nil {
			_, _ = io.Copy(c.Runner.Stdout, &stdout)
		}
		if c.Runner.Stderr != nil {
			_, _ = io.Copy(c.Runner.Stderr, &stderr)
		}
	}
	if err != nil {
		return err
	}

	return flushErr
}

func composeArgs(cfg configpkg.Config, subcommand string, extra ...string) []string {
	return composeArgsForPath(configpkg.ComposePath(cfg), subcommand, extra...)
}

func composeArgsForPath(composePath, subcommand string, extra ...string) []string {
	args := []string{
		"compose",
		"-f",
		composePath,
		subcommand,
	}

	return append(args, extra...)
}

func parseComposeContainerOutput(stdout string) ([]system.Container, error) {
	cleaned := strings.TrimSpace(cleanComposeOutput(stdout))
	if cleaned == "" {
		return []system.Container{}, nil
	}

	if strings.HasPrefix(cleaned, "[") {
		var entries []composeContainer
		if err := json.Unmarshal([]byte(cleaned), &entries); err != nil {
			return nil, fmt.Errorf("parse compose status output: %w", err)
		}
		return mapComposeContainers(entries), nil
	}

	decoder := json.NewDecoder(strings.NewReader(cleaned))
	containers := make([]system.Container, 0)
	for {
		var entry composeContainer
		if err := decoder.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse compose status output: %w", err)
		}
		containers = append(containers, entry.toSystemContainer())
	}

	return containers, nil
}

func mapComposeContainers(entries []composeContainer) []system.Container {
	containers := make([]system.Container, 0, len(entries))
	for _, entry := range entries {
		containers = append(containers, entry.toSystemContainer())
	}
	return containers
}

func (c composeContainer) toSystemContainer() system.Container {
	id := strings.TrimSpace(c.ID)
	if id == "" {
		id = strings.TrimSpace(c.LegacyID)
	}

	names := make([]string, 0, len(c.Names))
	for _, name := range c.Names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		names = append(names, trimmed)
	}
	if len(names) == 0 {
		trimmed := strings.TrimSpace(c.Name)
		if trimmed != "" {
			names = append(names, trimmed)
		}
	}

	ports := make([]system.ContainerPort, 0, len(c.Publishers))
	for _, publisher := range c.Publishers {
		ports = append(ports, system.ContainerPort{
			HostPort:      publisher.PublishedPort,
			ContainerPort: publisher.TargetPort,
			Protocol:      strings.ToLower(strings.TrimSpace(publisher.Protocol)),
		})
	}
	if len(ports) == 0 && len(c.Ports) > 0 {
		ports = append(ports, []system.ContainerPort(c.Ports)...)
	}

	return system.Container{
		ID:        id,
		Image:     c.Image,
		Names:     names,
		Status:    c.Status,
		State:     c.State,
		Ports:     ports,
		CreatedAt: c.CreatedAt,
	}
}

func filteredRunner(runner system.Runner) (system.Runner, func() error) {
	stdout := newComposeNoiseFilter(runner.Stdout)
	stderr := newComposeNoiseFilter(runner.Stderr)

	return system.Runner{
			Stdout: stdout,
			Stderr: stderr,
		}, func() error {
			if err := stdout.Flush(); err != nil {
				return err
			}
			return stderr.Flush()
		}
}

type composeNoiseFilter struct {
	target io.Writer
	buffer bytes.Buffer
}

func newComposeNoiseFilter(target io.Writer) *composeNoiseFilter {
	return &composeNoiseFilter{target: target}
}

func (f *composeNoiseFilter) Write(p []byte) (int, error) {
	if f.target == nil {
		return len(p), nil
	}

	if _, err := f.buffer.Write(p); err != nil {
		return 0, err
	}

	if err := f.writeReadyLines(); err != nil {
		return 0, err
	}

	return len(p), nil
}

func (f *composeNoiseFilter) Flush() error {
	if f.target == nil {
		return nil
	}
	if f.buffer.Len() == 0 {
		return nil
	}
	if shouldSkipComposeLine(f.buffer.String()) {
		f.buffer.Reset()
		return nil
	}
	_, err := io.Copy(f.target, &f.buffer)
	return err
}

func (f *composeNoiseFilter) writeReadyLines() error {
	for {
		data := f.buffer.Bytes()
		index := bytes.IndexByte(data, '\n')
		if index < 0 {
			return nil
		}

		line := string(data[:index])
		f.buffer.Next(index + 1)
		if shouldSkipComposeLine(line) {
			continue
		}
		if _, err := io.WriteString(f.target, line+"\n"); err != nil {
			return err
		}
	}
}

func shouldSkipComposeLine(line string) bool {
	cleaned := strings.TrimSpace(composeANSIPattern.ReplaceAllString(line, ""))
	return cleaned == "" || strings.HasPrefix(cleaned, ">>>> Executing external compose provider")
}

func cleanComposeOutput(raw string) string {
	lines := strings.Split(raw, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		cleaned := strings.TrimSpace(composeANSIPattern.ReplaceAllString(line, ""))
		if shouldSkipComposeLine(cleaned) {
			continue
		}
		filtered = append(filtered, cleaned)
	}

	return strings.Join(filtered, "\n")
}

var composeANSIPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
