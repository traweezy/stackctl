package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/logging"
	"github.com/traweezy/stackctl/internal/system"
)

type exactWriteErrorWriter struct {
	bytes.Buffer
	target string
}

func (w *exactWriteErrorWriter) Write(p []byte) (int, error) {
	if string(p) == w.target {
		return 0, errors.New("write failed")
	}
	return w.Buffer.Write(p)
}

type substringWriteErrorWriter struct {
	bytes.Buffer
	target string
}

func (w *substringWriteErrorWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), w.target) {
		return 0, errors.New("write failed")
	}
	return w.Buffer.Write(p)
}

func executeRootWithIO(t *testing.T, stdout io.Writer, stderr io.Writer, args ...string) error {
	t.Helper()

	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	root := NewRootCmd(NewApp())
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	envSnapshot := snapshotEnv(
		configpkg.StackNameEnvVar,
		"ACCESSIBLE",
		"STACKCTL_WIZARD_PLAIN",
		logging.EnvLogLevel,
		logging.EnvLogFormat,
		logging.EnvLogFile,
		logging.EnvTUIDebugLogFile,
	)
	originalOutput := rootOutput
	t.Cleanup(func() {
		restoreEnv(envSnapshot)
		logging.Reset()
		rootOutput = originalOutput
	})

	return root.Execute()
}

func TestCoverageBatchHealthConnectExecStopStart(t *testing.T) {
	t.Run("health helper constructors exercise the default closures", func(t *testing.T) {
		ctx, cancel := healthNotifyContext(context.Background(), os.Interrupt)
		cancel()
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected canceling the health notify context to close it")
		}

		tickC, stop := newHealthTicker(time.Millisecond)
		defer stop()

		select {
		case <-tickC:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected health ticker to fire")
		}
	})

	t.Run("health propagates status write failures", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
		})

		if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "health"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected health write failure, got %v", err)
		}
	})

	t.Run("health watch propagates newline write failures", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: "[]"}, nil
			}
		})

		originalNotifyContext := healthNotifyContext
		originalTicker := newHealthTicker
		t.Cleanup(func() {
			healthNotifyContext = originalNotifyContext
			newHealthTicker = originalTicker
		})

		ctx, cancel := context.WithCancel(context.Background())
		healthNotifyContext = func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
			return ctx, cancel
		}

		tickC := make(chan time.Time, 1)
		newHealthTicker = func(time.Duration) (<-chan time.Time, func()) {
			return tickC, func() {}
		}

		cmd := newHealthCmd()
		if err := cmd.Flags().Set("watch", "true"); err != nil {
			t.Fatalf("set watch flag: %v", err)
		}
		cmd.SetOut(&exactWriteErrorWriter{target: "\n"})

		done := make(chan error, 1)
		go func() {
			done <- cmd.RunE(cmd, nil)
		}()

		tickC <- time.Now()
		err := <-done
		cancel()
		if err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected health watch newline failure, got %v", err)
		}
	})

	t.Run("connect propagates runtime config failures", func(t *testing.T) {
		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) {
				return configpkg.Config{}, errors.New("load failed")
			}
		})

		_, _, err := executeRoot(t, "connect")
		if err == nil || !strings.Contains(err.Error(), "load failed") {
			t.Fatalf("expected connect load failure, got %v", err)
		}
	})

	t.Run("exec covers invalid service, compose runtime, verbose writer, and signal cancellation", func(t *testing.T) {
		t.Run("invalid service", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			})

			_, _, err := executeRoot(t, "exec", "bogus", "--", "sh")
			if err == nil || !strings.Contains(err.Error(), `invalid service "bogus"`) {
				t.Fatalf("expected invalid exec service error, got %v", err)
			}
		})

		t.Run("compose runtime unavailable", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
				d.podmanComposeAvail = func(context.Context) bool { return false }
			})

			_, _, err := executeRoot(t, "exec", "postgres", "--", "psql")
			if err == nil || !strings.Contains(err.Error(), "podman compose is not available") {
				t.Fatalf("expected compose runtime error, got %v", err)
			}
		})

		t.Run("verbose compose file write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
			})

			if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "--verbose", "exec", "postgres", "--", "psql"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected verbose compose write failure, got %v", err)
			}
		})

		t.Run("signal cancellation suppresses exec errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return configpkg.Default(), nil }
				d.composeExec = func(context.Context, system.Runner, configpkg.Config, string, []string, []string, bool) error {
					_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
					time.Sleep(20 * time.Millisecond)
					return errors.New("ignored after signal")
				}
			})

			_, _, err := executeRoot(t, "exec", "postgres", "--", "psql")
			if err != nil {
				t.Fatalf("expected exec signal cancellation to return nil, got %v", err)
			}
		})
	})

	t.Run("stop covers status, full-stack, and service error branches", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		t.Run("status writer failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			})

			if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "stop", "redis"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected stop status write failure, got %v", err)
			}
		})

		t.Run("compose down error", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
					return errors.New("compose down failed")
				}
			})

			_, _, err := executeRoot(t, "stop")
			if err == nil || !strings.Contains(err.Error(), "compose down failed") {
				t.Fatalf("expected stop compose-down error, got %v", err)
			}
		})

		t.Run("service stop error", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.composeStopServices = func(context.Context, system.Runner, configpkg.Config, []string) error {
					return errors.New("stop failed")
				}
			})

			_, _, err := executeRoot(t, "stop", "redis")
			if err == nil || !strings.Contains(err.Error(), "stop failed") {
				t.Fatalf("expected stop services error, got %v", err)
			}
		})
	})

	t.Run("start covers scaffold, wait, verify, and blank-line failure branches", func(t *testing.T) {
		t.Run("managed scaffold inspection failure", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.managedStackNeedsScaffold = func(configpkg.Config) (bool, error) {
					return false, errors.New("scaffold boom")
				}
			})

			_, _, err := executeRoot(t, "start")
			if err == nil || !strings.Contains(err.Error(), "scaffold boom") {
				t.Fatalf("expected start scaffold error, got %v", err)
			}
		})

		t.Run("wait-for-services failure", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.Behavior.WaitForServicesStart = true
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
				}
				d.waitForPort = func(context.Context, int, time.Duration) error {
					return errors.New("ready timeout")
				}
			})

			_, _, err := executeRoot(t, "start", "postgres")
			if err == nil || !strings.Contains(err.Error(), "ready timeout") {
				t.Fatalf("expected start wait failure, got %v", err)
			}
		})

		t.Run("non-wait verification failure", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.Behavior.WaitForServicesStart = false
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{
						Stdout: marshalContainersJSON(system.Container{
							Names:  []string{cfg.Services.PostgresContainer},
							State:  "exited",
							Status: "Exited (1) 2 seconds ago",
						}),
					}, nil
				}
			})

			_, _, err := executeRoot(t, "start", "postgres")
			if err == nil || !strings.Contains(err.Error(), "container failed to start") {
				t.Fatalf("expected start verification failure, got %v", err)
			}
		})

		t.Run("blank line write failure", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.Behavior.WaitForServicesStart = true
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
				}
				d.waitForPort = func(context.Context, int, time.Duration) error { return nil }
			})

			if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 3}, io.Discard, "start", "postgres"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected start blank-line write failure, got %v", err)
			}
		})
	})
}

func TestCoverageBatchStatusServicesLogsOpenResetAndLifecycle(t *testing.T) {
	t.Run("status propagates JSON write failures", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		withTestDeps(t, func(d *commandDeps) {
			d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
				return system.CommandResult{Stdout: runningContainerJSON(cfg)}, nil
			}
		})

		if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "status", "--json"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected status JSON write failure, got %v", err)
		}
		if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 2}, io.Discard, "status", "--json"); err == nil || !strings.Contains(err.Error(), "write failed") {
			t.Fatalf("expected status JSON newline write failure, got %v", err)
		}
	})

	t.Run("services covers copy conflict, clipboard errors, and copy status writes", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		t.Run("json copy conflict", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			})

			_, _, err := executeRoot(t, "services", "--json", "--copy", "postgres")
			if err == nil || !strings.Contains(err.Error(), "--json and --copy cannot be used together") {
				t.Fatalf("expected services flag conflict, got %v", err)
			}
		})

		t.Run("clipboard error", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.copyToClipboard = func(context.Context, system.Runner, string) error {
					return errors.New("clipboard boom")
				}
			})

			_, _, err := executeRoot(t, "services", "--copy", "postgres")
			if err == nil || !strings.Contains(err.Error(), "clipboard boom") {
				t.Fatalf("expected services clipboard error, got %v", err)
			}
		})

		t.Run("copy status write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			})

			if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "services", "--copy", "postgres"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected services copy write failure, got %v", err)
			}
		})
	})

	t.Run("logs covers compose-runtime errors and signal cancellation", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		t.Run("compose runtime unavailable", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.podmanComposeAvail = func(context.Context) bool { return false }
			})

			_, _, err := executeRoot(t, "logs")
			if err == nil || !strings.Contains(err.Error(), "podman compose is not available") {
				t.Fatalf("expected logs compose-runtime error, got %v", err)
			}
		})

		t.Run("watch signal cancellation suppresses log errors", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.composeLogs = func(context.Context, system.Runner, configpkg.Config, int, bool, string, string) error {
					_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
					time.Sleep(20 * time.Millisecond)
					return errors.New("ignored after signal")
				}
			})

			_, _, err := executeRoot(t, "logs", "--watch")
			if err != nil {
				t.Fatalf("expected logs signal cancellation to return nil, got %v", err)
			}
		})
	})

	t.Run("open all propagates fallback writer failures for cockpit and meilisearch", func(t *testing.T) {
		baseCfg := configpkg.Default()
		baseCfg.Setup.IncludeCockpit = true
		baseCfg.Setup.IncludeMeilisearch = true
		baseCfg.Setup.IncludePgAdmin = true
		baseCfg.ApplyDerivedFields()

		t.Run("cockpit fallback write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return baseCfg, nil }
				d.openURL = func(context.Context, system.Runner, string) error { return errors.New("no opener") }
			})

			if err := executeRootWithIO(t, &substringWriteErrorWriter{target: "could not open cockpit automatically"}, io.Discard, "open", "all"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected cockpit fallback write failure, got %v", err)
			}
		})

		t.Run("meilisearch fallback write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return baseCfg, nil }
				d.openURL = func(_ context.Context, _ system.Runner, target string) error {
					if target == baseCfg.URLs.Cockpit {
						return nil
					}
					return errors.New("no opener")
				}
			})

			if err := executeRootWithIO(t, &substringWriteErrorWriter{target: "could not open meilisearch automatically"}, io.Discard, "open", "all"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected meilisearch fallback write failure, got %v", err)
			}
		})
	})

	t.Run("reset covers prompt, status write, and compose-down failures", func(t *testing.T) {
		cfg := configpkg.Default()
		cfg.ApplyDerivedFields()

		t.Run("prompt unavailable without force", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.isTerminal = func() bool { return true }
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.promptYesNo = func(io.Reader, io.Writer, string, bool) (bool, error) {
					return false, io.ErrUnexpectedEOF
				}
			})

			_, _, err := executeRoot(t, "reset", "--volumes")
			if err == nil || !strings.Contains(err.Error(), "volume wipe confirmation required; rerun with --force") {
				t.Fatalf("expected reset prompt error, got %v", err)
			}
		})

		t.Run("status write failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
			})

			if err := executeRootWithIO(t, &failingWriteBuffer{failAfter: 1}, io.Discard, "reset", "--force"); err == nil || !strings.Contains(err.Error(), "write failed") {
				t.Fatalf("expected reset status write failure, got %v", err)
			}
		})

		t.Run("compose down failure", func(t *testing.T) {
			withTestDeps(t, func(d *commandDeps) {
				d.loadConfig = func(string) (configpkg.Config, error) { return cfg, nil }
				d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
					return errors.New("compose down failed")
				}
			})

			_, _, err := executeRoot(t, "reset", "--force")
			if err == nil || !strings.Contains(err.Error(), "compose down failed") {
				t.Fatalf("expected reset compose-down error, got %v", err)
			}
		})
	})

	t.Run("lifecycle helpers cover disabled targets, runtime failures, and active-stack guards", func(t *testing.T) {
		t.Run("resolveTargetStackServices rejects disabled services", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.Setup.IncludeSeaweedFS = false
			cfg.ApplyDerivedFields()

			_, err := resolveTargetStackServices(cfg, []string{"seaweedfs"})
			if err == nil || !strings.Contains(err.Error(), "seaweedfs is not enabled") {
				t.Fatalf("expected disabled service error, got %v", err)
			}
		})

		t.Run("ensureSelectedServicePortsAvailable reports port probe errors and conflicts", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: "[]"}, nil
				}
				d.portInUse = func(int) (bool, error) { return false, errors.New("probe failed") }
			})

			err := ensureSelectedServicePortsAvailable(context.Background(), cfg, []string{"postgres"})
			if err == nil || !strings.Contains(err.Error(), "probe failed") {
				t.Fatalf("expected port probe error, got %v", err)
			}

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: "[]"}, nil
				}
				d.portInUse = func(int) (bool, error) { return true, nil }
			})

			err = ensureSelectedServicePortsAvailable(context.Background(), cfg, []string{"postgres"})
			if err == nil || !strings.Contains(err.Error(), "cannot start postgres") {
				t.Fatalf("expected port conflict error, got %v", err)
			}
		})

		t.Run("verifySelectedServicesStarted reports runtime and terminal-container failures", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{}, errors.New("podman ps failed")
				}
			})

			err := verifySelectedServicesStarted(context.Background(), cfg, []string{"postgres"})
			if err == nil || !strings.Contains(err.Error(), "podman ps failed") {
				t.Fatalf("expected verify runtime error, got %v", err)
			}

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{
						Stdout: marshalContainersJSON(system.Container{
							Names:  []string{cfg.Services.PostgresContainer},
							State:  "exited",
							Status: "Exited (1) 2 seconds ago",
						}),
					}, nil
				}
			})

			err = verifySelectedServicesStarted(context.Background(), cfg, []string{"postgres"})
			if err == nil || !strings.Contains(err.Error(), "container failed to start") {
				t.Fatalf("expected verify container failure, got %v", err)
			}
		})

		t.Run("waitForStackService surfaces context cancellation when startup never completes", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()

			definition, ok := serviceDefinitionByKey("postgres")
			if !ok {
				t.Fatal("expected postgres service definition")
			}

			withTestDeps(t, func(d *commandDeps) {
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: "[]"}, nil
				}
			})

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := waitForStackService(ctx, cfg, definition)
			if err == nil || !strings.Contains(err.Error(), "did not become ready") {
				t.Fatalf("expected waitForStackService cancellation error, got %v", err)
			}
		})

		t.Run("ensureNoOtherRunningStack and helpers report path and active-stack errors", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.Stack.Name = "staging"
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.configFilePath = func() (string, error) { return "", errors.New("config path boom") }
			})

			err := ensureNoOtherRunningStack(context.Background())
			if err == nil || !strings.Contains(err.Error(), "config path boom") {
				t.Fatalf("expected current config path error, got %v", err)
			}

			withTestDeps(t, func(d *commandDeps) {
				d.configFilePath = func() (string, error) { return "/tmp/stackctl/config.yaml", nil }
				d.knownConfigPaths = func() ([]string, error) {
					return nil, errors.New("known config boom")
				}
			})

			_, err = otherRunningLocalStack(context.Background(), "/tmp/stackctl/config.yaml")
			if err == nil || !strings.Contains(err.Error(), "known config boom") {
				t.Fatalf("expected known-configs error, got %v", err)
			}

			withTestDeps(t, func(d *commandDeps) {
				d.configFilePath = func() (string, error) { return "/tmp/stackctl/config.yaml", nil }
				d.knownConfigPaths = func() ([]string, error) {
					return []string{"/tmp/stackctl/config.yaml", "/tmp/stackctl/staging.yaml"}, nil
				}
				d.loadConfig = func(path string) (configpkg.Config, error) {
					if strings.Contains(path, "staging") {
						return cfg, nil
					}
					return configpkg.Default(), nil
				}
				d.captureResult = func(context.Context, string, string, ...string) (system.CommandResult, error) {
					return system.CommandResult{Stdout: runningContainerJSON(cfg, "postgres")}, nil
				}
			})

			err = ensureNoOtherRunningStack(context.Background())
			if err == nil || !strings.Contains(err.Error(), "another local stack is already running: staging") || !strings.Contains(err.Error(), "Postgres") {
				t.Fatalf("expected active other-stack error, got %v", err)
			}
		})

		t.Run("composeDownAndWait surfaces compose-down failures", func(t *testing.T) {
			cfg := configpkg.Default()
			cfg.ApplyDerivedFields()

			withTestDeps(t, func(d *commandDeps) {
				d.composeDown = func(context.Context, system.Runner, configpkg.Config, bool) error {
					return errors.New("compose down failed")
				}
			})

			err := composeDownAndWait(context.Background(), system.Runner{}, cfg, false)
			if err == nil || !strings.Contains(err.Error(), "compose down failed") {
				t.Fatalf("expected composeDownAndWait error, got %v", err)
			}
		})
	})
}
