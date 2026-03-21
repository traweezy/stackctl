package compose

import (
	"bytes"
	"context"
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

func (c Client) Up(ctx context.Context, cfg configpkg.Config) error {
	return c.runCompose(ctx, cfg.Stack.Dir, composeArgs(cfg, "up", "-d")...)
}

func (c Client) Down(ctx context.Context, cfg configpkg.Config, removeVolumes bool) error {
	args := composeArgs(cfg, "down")
	if removeVolumes {
		args = append(args, "-v")
	}

	return c.runCompose(ctx, cfg.Stack.Dir, args...)
}

func (c Client) Logs(ctx context.Context, cfg configpkg.Config, tail int, follow bool, since string) error {
	args := composeArgs(cfg, "logs")
	if follow {
		args = append(args, "-f")
	}
	args = append(args, "--tail", strconv.Itoa(tail))
	if since != "" {
		args = append(args, "--since", since)
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

func (c Client) runCompose(ctx context.Context, dir string, args ...string) error {
	runner, flush := filteredRunner(c.Runner)
	err := runner.Run(ctx, dir, "podman", args...)
	flushErr := flush()
	if err != nil {
		return err
	}

	return flushErr
}

func composeArgs(cfg configpkg.Config, subcommand string, extra ...string) []string {
	args := []string{
		"compose",
		"-f",
		configpkg.ComposePath(cfg),
		subcommand,
	}

	return append(args, extra...)
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

var composeANSIPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
