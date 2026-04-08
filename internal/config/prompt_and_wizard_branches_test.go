package config

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/traweezy/stackctl/internal/system"
)

type failOnSubstringWriter struct {
	failOn string
	err    error
}

func (w *failOnSubstringWriter) Write(p []byte) (int, error) {
	if w.failOn == "" || strings.Contains(string(p), w.failOn) {
		return 0, w.err
	}
	return len(p), nil
}

func minimalPromptCoveragePlatform() system.Platform {
	return system.Platform{
		GOOS:           "linux",
		PackageManager: "apt",
		ServiceManager: system.ServiceManagerSystemd,
	}
}

func minimalPromptCoverageBase() Config {
	base := DefaultForStackOnPlatform("dev-stack", minimalPromptCoveragePlatform())
	base.Setup.IncludeRedis = false
	base.Setup.IncludeNATS = false
	base.Setup.IncludeSeaweedFS = false
	base.Setup.IncludeMeilisearch = false
	base.Setup.IncludePgAdmin = false
	base.Setup.IncludeCockpit = false
	return base
}

func minimalPromptCoverageAnswers() string {
	return wizardAnswers(
		"", // stack name
		"", // manage stack
		"", // include postgres
		"", // postgres container
		"", // postgres image
		"", // postgres data volume
		"", // postgres maintenance database
		"", // postgres port
		"", // postgres database
		"", // postgres username
		"", // postgres password
		"", // include redis
		"", // include nats
		"", // include seaweedfs
		"", // include meilisearch
		"", // include pgadmin
		"", // include cockpit
		"", // wait for services
		"", // startup timeout
		"", // package manager
	)
}

func withDeletedWorkingDir(t *testing.T, fn func()) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) returned error: %v", dir, err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll(%q) returned error: %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory %q: %v", wd, err)
		}
	}()

	fn()
}

func TestPromptCoverageLargeBatchWriteErrorBranches(t *testing.T) {
	errBoom := errors.New("write boom")
	unsupportedCockpit := system.Platform{
		GOOS:           "darwin",
		PackageManager: "brew",
		ServiceManager: system.ServiceManagerNone,
	}

	cases := []struct {
		name   string
		failOn string
		run    func(io.Writer) error
	}{
		{
			name:   "postgres section write",
			failOn: "[Postgres]",
			run: func(out io.Writer) error {
				cfg := Default()
				cfg.Setup.IncludePostgres = true
				return (promptSession{reader: bufioReaderFor("\n"), out: out}).configurePostgres(&cfg)
			},
		},
		{
			name:   "redis section write",
			failOn: "[Redis]",
			run: func(out io.Writer) error {
				cfg := Default()
				cfg.Setup.IncludeRedis = true
				return (promptSession{reader: bufioReaderFor("\n"), out: out}).configureRedis(&cfg)
			},
		},
		{
			name:   "nats section write",
			failOn: "[NATS]",
			run: func(out io.Writer) error {
				cfg := Default()
				cfg.Setup.IncludeNATS = true
				return (promptSession{reader: bufioReaderFor("\n"), out: out}).configureNATS(&cfg)
			},
		},
		{
			name:   "seaweedfs section write",
			failOn: "[SeaweedFS]",
			run: func(out io.Writer) error {
				cfg := Default()
				cfg.Setup.IncludeSeaweedFS = true
				return (promptSession{reader: bufioReaderFor("\n"), out: out}).configureSeaweedFS(&cfg)
			},
		},
		{
			name:   "pgadmin section write",
			failOn: "[pgAdmin]",
			run: func(out io.Writer) error {
				cfg := Default()
				cfg.Setup.IncludePgAdmin = true
				return (promptSession{reader: bufioReaderFor("\n"), out: out}).configurePgAdmin(&cfg)
			},
		},
		{
			name:   "meilisearch section write",
			failOn: "[Meilisearch]",
			run: func(out io.Writer) error {
				cfg := Default()
				cfg.Setup.IncludeMeilisearch = true
				return (promptSession{reader: bufioReaderFor("\n"), out: out}).configureMeilisearch(&cfg)
			},
		},
		{
			name:   "cockpit section write",
			failOn: "[Cockpit]",
			run: func(out io.Writer) error {
				cfg := DefaultForStackOnPlatform("dev-stack", unsupportedCockpit)
				cfg.Setup.IncludeCockpit = true
				return (promptSession{reader: bufioReaderFor("\n"), out: out}).configureCockpit(&cfg, unsupportedCockpit)
			},
		},
		{
			name:   "cockpit helper note write",
			failOn: "Enable Cockpit in stackctl helper output",
			run: func(out io.Writer) error {
				cfg := DefaultForStackOnPlatform("dev-stack", unsupportedCockpit)
				cfg.Setup.IncludeCockpit = true
				return (promptSession{reader: bufioReaderFor("\n"), out: out}).configureCockpit(&cfg, unsupportedCockpit)
			},
		},
		{
			name:   "cockpit install note write",
			failOn: "does not support Cockpit installation in stackctl",
			run: func(out io.Writer) error {
				cfg := DefaultForStackOnPlatform("dev-stack", unsupportedCockpit)
				cfg.Setup.IncludeCockpit = true
				return (promptSession{reader: bufioReaderFor("\n\n"), out: out}).configureCockpit(&cfg, unsupportedCockpit)
			},
		},
		{
			name:   "behavior section write",
			failOn: "[Behavior]",
			run: func(out io.Writer) error {
				_, err := runPlainWizardWithPlatform(strings.NewReader(minimalPromptCoverageAnswers()), out, minimalPromptCoverageBase(), minimalPromptCoveragePlatform())
				return err
			},
		},
		{
			name:   "system section write",
			failOn: "[System]",
			run: func(out io.Writer) error {
				_, err := runPlainWizardWithPlatform(strings.NewReader(minimalPromptCoverageAnswers()), out, minimalPromptCoverageBase(), minimalPromptCoveragePlatform())
				return err
			},
		},
		{
			name:   "system note write",
			failOn: "The package manager stackctl should use",
			run: func(out io.Writer) error {
				_, err := runPlainWizardWithPlatform(strings.NewReader(minimalPromptCoverageAnswers()), out, minimalPromptCoverageBase(), minimalPromptCoveragePlatform())
				return err
			},
		},
		{
			name:   "system package manager prompt write",
			failOn: "Package manager [apt]: ",
			run: func(out io.Writer) error {
				_, err := runPlainWizardWithPlatform(strings.NewReader(minimalPromptCoverageAnswers()), out, minimalPromptCoverageBase(), minimalPromptCoveragePlatform())
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(&failOnSubstringWriter{failOn: tc.failOn, err: errBoom})
			if !errors.Is(err, errBoom) {
				t.Fatalf("expected %q to return %v, got %v", tc.name, errBoom, err)
			}
		})
	}
}

func TestPromptCoverageLargeBatchReadAndValidationBranches(t *testing.T) {
	errBoom := errors.New("boom")

	t.Run("askString prompt write error", func(t *testing.T) {
		session := promptSession{reader: bufioReaderFor("value\n"), out: &failOnSubstringWriter{err: errBoom}}
		if _, err := session.askString("Name", "", nonEmpty); !errors.Is(err, errBoom) {
			t.Fatalf("expected askString write error, got %v", err)
		}
	})

	t.Run("askString validation write error", func(t *testing.T) {
		session := promptSession{
			reader: bufioReaderFor("\n"),
			out:    &failOnSubstringWriter{failOn: "value must not be empty", err: errBoom},
		}
		if _, err := session.askString("Name", "", nonEmpty); !errors.Is(err, errBoom) {
			t.Fatalf("expected askString validation write error, got %v", err)
		}
	})

	t.Run("askString validation EOF returns validation error", func(t *testing.T) {
		session := promptSession{
			reader: bufioReaderFor(" "),
			out:    io.Discard,
		}
		if _, err := session.askString("Name", "", nonEmpty); err == nil || !strings.Contains(err.Error(), "value must not be empty") {
			t.Fatalf("expected askString EOF validation error, got %v", err)
		}
	})

	t.Run("askInt invalid-number write error", func(t *testing.T) {
		session := promptSession{
			reader: bufioReaderFor("oops\n"),
			out:    &failOnSubstringWriter{failOn: "Enter a valid number.", err: errBoom},
		}
		if _, err := session.askInt("Timeout", 30, positiveInt); !errors.Is(err, errBoom) {
			t.Fatalf("expected askInt invalid-number write error, got %v", err)
		}
	})

	t.Run("askInt validation write error", func(t *testing.T) {
		session := promptSession{
			reader: bufioReaderFor("0\n"),
			out:    &failOnSubstringWriter{failOn: "value must be greater than zero", err: errBoom},
		}
		if _, err := session.askInt("Timeout", 30, positiveInt); !errors.Is(err, errBoom) {
			t.Fatalf("expected askInt validation write error, got %v", err)
		}
	})

	t.Run("askBool prompt write error", func(t *testing.T) {
		session := promptSession{reader: bufioReaderFor("y\n"), out: &failOnSubstringWriter{err: errBoom}}
		if _, err := session.askBool("Continue", false); !errors.Is(err, errBoom) {
			t.Fatalf("expected askBool prompt write error, got %v", err)
		}
	})

	t.Run("askBool invalid-answer write error", func(t *testing.T) {
		session := promptSession{
			reader: bufioReaderFor("maybe\n"),
			out:    &failOnSubstringWriter{failOn: "Enter y or n.", err: errBoom},
		}
		if _, err := session.askBool("Continue", false); !errors.Is(err, errBoom) {
			t.Fatalf("expected askBool invalid-answer write error, got %v", err)
		}
	})

	t.Run("askStackDir confirmation read error", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		session := promptSession{
			reader: ioReaderToPromptSession(&failingLinesReader{remaining: 1, err: errBoom}),
			out:    io.Discard,
		}
		if _, err := session.askStackDir(missing); !errors.Is(err, errBoom) {
			t.Fatalf("expected askStackDir confirmation read error, got %v", err)
		}
	})

	t.Run("askStackDir abs error from missing working directory", func(t *testing.T) {
		withDeletedWorkingDir(t, func() {
			session := promptSession{reader: bufioReaderFor("relative\n"), out: io.Discard}
			if _, err := session.askStackDir("relative"); err == nil || !strings.Contains(err.Error(), "resolve stack directory") {
				t.Fatalf("expected askStackDir abs error, got %v", err)
			}
		})
	})

	t.Run("validStackName hits non-empty branch", func(t *testing.T) {
		if err := validStackName("   "); err == nil || !strings.Contains(err.Error(), "value must not be empty") {
			t.Fatalf("expected blank stack name to fail, got %v", err)
		}
	})

	seaweedCases := []struct {
		name      string
		remaining int
	}{
		{name: "seaweed container read", remaining: 1},
		{name: "seaweed image read", remaining: 2},
		{name: "seaweed data volume read", remaining: 3},
		{name: "seaweed volume limit read", remaining: 4},
		{name: "seaweed port read", remaining: 5},
		{name: "seaweed access key read", remaining: 6},
		{name: "seaweed secret key read", remaining: 7},
	}
	for _, tc := range seaweedCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			cfg.Setup.IncludeSeaweedFS = true
			session := promptSession{
				reader: ioReaderToPromptSession(&failingLinesReader{remaining: tc.remaining, err: errBoom}),
				out:    io.Discard,
			}
			if err := session.configureSeaweedFS(&cfg); !errors.Is(err, errBoom) {
				t.Fatalf("expected SeaweedFS read error, got %v", err)
			}
		})
	}

	meiliCases := []struct {
		name      string
		remaining int
	}{
		{name: "meilisearch container read", remaining: 1},
		{name: "meilisearch image read", remaining: 2},
		{name: "meilisearch data volume read", remaining: 3},
		{name: "meilisearch port read", remaining: 4},
		{name: "meilisearch master key read", remaining: 5},
	}
	for _, tc := range meiliCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			cfg.Setup.IncludeMeilisearch = true
			session := promptSession{
				reader: ioReaderToPromptSession(&failingLinesReader{remaining: tc.remaining, err: errBoom}),
				out:    io.Discard,
			}
			if err := session.configureMeilisearch(&cfg); !errors.Is(err, errBoom) {
				t.Fatalf("expected Meilisearch read error, got %v", err)
			}
		})
	}
}

func ioReaderToPromptSession(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}

func TestWizardStateCoverageLargeBatchConversionAndHelperBranches(t *testing.T) {
	base := Default()
	platform := system.Platform{
		GOOS:           "linux",
		PackageManager: "dnf",
		ServiceManager: system.ServiceManagerSystemd,
	}

	cases := []struct {
		name   string
		mutate func(*wizardState)
		want   string
	}{
		{
			name: "managed invalid stack name",
			mutate: func(state *wizardState) {
				state.StackMode = wizardStackModeManaged
				state.StackName = "invalid!"
			},
			want: "stack name",
		},
		{
			name: "startup timeout parse error",
			mutate: func(state *wizardState) {
				state.StartupTimeoutSec = "0"
			},
			want: "startup timeout",
		},
		{
			name: "postgres max connections parse error",
			mutate: func(state *wizardState) {
				state.PostgresMaxConnections = "0"
			},
			want: "postgres max connections",
		},
		{
			name: "postgres log duration parse error",
			mutate: func(state *wizardState) {
				state.PostgresLogDurationMS = "0"
			},
			want: "postgres log min duration",
		},
		{
			name: "postgres port parse error",
			mutate: func(state *wizardState) {
				state.PostgresPort = "70000"
			},
			want: "postgres port",
		},
		{
			name: "redis port parse error",
			mutate: func(state *wizardState) {
				state.RedisPort = "70000"
			},
			want: "redis port",
		},
		{
			name: "nats port parse error",
			mutate: func(state *wizardState) {
				state.NATSPort = "70000"
			},
			want: "nats port",
		},
		{
			name: "seaweed size parse error",
			mutate: func(state *wizardState) {
				state.SeaweedFSVolumeSizeMB = "0"
			},
			want: "seaweedfs volume size limit",
		},
		{
			name: "seaweed port parse error",
			mutate: func(state *wizardState) {
				state.SeaweedFSPort = "70000"
			},
			want: "seaweedfs port",
		},
		{
			name: "meilisearch port parse error",
			mutate: func(state *wizardState) {
				state.MeilisearchPort = "70000"
			},
			want: "meilisearch port",
		},
		{
			name: "pgadmin port parse error",
			mutate: func(state *wizardState) {
				state.PgAdminPort = "70000"
			},
			want: "pgadmin port",
		},
		{
			name: "cockpit port parse error",
			mutate: func(state *wizardState) {
				state.CockpitPort = "70000"
			},
			want: "cockpit port",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := newWizardState(base)
			tc.mutate(&state)
			if _, err := state.toConfigForPlatform(base, platform); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}

	t.Run("external stack abs failure from missing working directory", func(t *testing.T) {
		withDeletedWorkingDir(t, func() {
			state := newWizardState(base)
			state.StackMode = wizardStackModeExternal
			state.ExternalStackDir = "relative"
			state.ExternalComposeFile = "compose.yaml"
			if _, err := state.toConfigForPlatform(base, platform); err == nil || !strings.Contains(err.Error(), "resolve stack directory") {
				t.Fatalf("expected external stack abs error, got %v", err)
			}
		})
	})

	t.Run("wizard helper branches in missing working directory", func(t *testing.T) {
		withDeletedWorkingDir(t, func() {
			if got := wizardFilePickerStartDir("   "); got != "." {
				t.Fatalf("expected blank picker start dir fallback '.', got %q", got)
			}
			if got := wizardFilePickerStartDir("relative"); got != "." {
				t.Fatalf("expected relative picker start dir fallback '.', got %q", got)
			}

			state := wizardState{
				StackMode:           wizardStackModeExternal,
				ExternalStackDir:    "relative",
				ExternalComposeFile: "compose.yaml",
			}
			if state.needsMissingExternalDirConfirmation() {
				t.Fatal("expected missing cwd path resolution to keep confirmation false")
			}
		})
	})

	t.Run("wizard step helpers missing target", func(t *testing.T) {
		state := newWizardState(base)
		if position, total := wizardStepPosition(&state, wizardStepID("missing")); position != 0 || total == 0 {
			t.Fatalf("expected missing wizard step position 0 with non-zero total, got position=%d total=%d", position, total)
		}
		if got := wizardNextStepLabel(&state, wizardStepID("missing")); got != "" {
			t.Fatalf("expected missing wizard next-step label to be blank, got %q", got)
		}
	})

	t.Run("parse helper empty and invalid branches", func(t *testing.T) {
		for name, fn := range map[string]func(string) (int, error){
			"parsePositiveInt empty":           parsePositiveInt,
			"parsePositiveInt invalid":         parsePositiveInt,
			"parsePostgresLogDuration empty":   parsePostgresLogDurationMS,
			"parsePostgresLogDuration invalid": parsePostgresLogDurationMS,
		} {
			value := ""
			if strings.Contains(name, "invalid") {
				value = "oops"
			}
			if _, err := fn(value); err == nil {
				t.Fatalf("%s: expected parse failure", name)
			}
		}
	})
}
