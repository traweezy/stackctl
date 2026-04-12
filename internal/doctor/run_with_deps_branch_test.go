package doctor

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	configpkg "github.com/traweezy/stackctl/internal/config"
	"github.com/traweezy/stackctl/internal/output"
	"github.com/traweezy/stackctl/internal/system"
)

func TestRunWithDepsAdditionalBranches(t *testing.T) {
	t.Run("config path and config load failures are returned", func(t *testing.T) {
		_, err := runWithDeps(context.Background(), dependencies{
			configFilePath: func() (string, error) { return "", errors.New("config path failed") },
		})
		if err == nil || !strings.Contains(err.Error(), "config path failed") {
			t.Fatalf("unexpected configFilePath error: %v", err)
		}

		_, err = runWithDeps(context.Background(), dependencies{
			configFilePath: func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
			loadConfig:     func(string) (configpkg.Config, error) { return configpkg.Config{}, errors.New("load failed") },
		})
		if err == nil || !strings.Contains(err.Error(), "load failed") {
			t.Fatalf("unexpected config load error: %v", err)
		}
	})

	t.Run("doctor reports missing files cockpit variants and port-check failures", func(t *testing.T) {
		cfg := configpkg.DefaultForStack("dev-stack")
		cfg.Setup.IncludeCockpit = true
		cfg.ApplyDerivedFields()

		report, err := runWithDeps(context.Background(), dependencies{
			configFilePath: func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
			loadConfig:     func(string) (configpkg.Config, error) { return cfg, nil },
			validateConfig: func(configpkg.Config) []configpkg.ValidationIssue { return nil },
			composePath:    func(configpkg.Config) string { return "/tmp/stackctl/compose.yaml" },
			stat: func(path string) (os.FileInfo, error) {
				return nil, errors.New("missing")
			},
			platform: func() system.Platform {
				return system.Platform{
					GOOS:           "linux",
					PackageManager: "brew",
					ServiceManager: system.ServiceManagerNone,
				}
			},
			commandExists:       func(string) bool { return false },
			podmanComposeAvail:  func(context.Context) bool { return false },
			podmanMachineStatus: func(context.Context) system.PodmanMachineState { return system.PodmanMachineState{} },
			openCommandName:     func() string { return "" },
			cockpitStatus: func(context.Context) system.CockpitState {
				return system.CockpitState{Installed: false, Active: false, State: "inactive"}
			},
			portInUse: func(port int) (bool, error) {
				if port == cfg.Ports.Cockpit {
					return false, errors.New("port check failed")
				}
				return false, nil
			},
			listContainers:  func(context.Context) ([]system.Container, error) { return nil, nil },
			redisOvercommit: func(context.Context) (system.OvercommitStatus, error) { return system.OvercommitStatus{}, nil },
		})
		if err != nil {
			t.Fatalf("runWithDeps returned error: %v", err)
		}

		for _, want := range []struct {
			status  string
			message string
		}{
			{status: output.StatusFail, message: "stack directory missing: " + cfg.Stack.Dir},
			{status: output.StatusFail, message: "compose file missing: /tmp/stackctl/compose.yaml"},
			{status: output.StatusWarn, message: "cockpit helpers are not supported on brew"},
		} {
			if !doctorHasCheck(report, want.status, want.message) {
				t.Fatalf("expected doctor check %s %q, got %+v", want.status, want.message, report.Checks)
			}
		}
	})

	t.Run("doctor reports cockpit active inactive and missing-install variants", func(t *testing.T) {
		baseCfg := configpkg.DefaultForStack("dev-stack")
		baseCfg.Setup.IncludeCockpit = true
		baseCfg.ApplyDerivedFields()

		testCases := []struct {
			name     string
			platform system.Platform
			cockpit  system.CockpitState
			want     string
		}{
			{
				name:     "inactive socket",
				platform: system.Platform{GOOS: "linux", PackageManager: "dnf", ServiceManager: system.ServiceManagerSystemd},
				cockpit:  system.CockpitState{Installed: true, Active: false, State: "inactive"},
				want:     "cockpit.socket inactive",
			},
			{
				name:     "autoinstall supported",
				platform: system.Platform{GOOS: "linux", PackageManager: "dnf", ServiceManager: system.ServiceManagerSystemd},
				cockpit:  system.CockpitState{Installed: false, Active: false},
				want:     "cockpit.socket not installed",
			},
			{
				name:     "active port check failure",
				platform: system.Platform{GOOS: "linux", PackageManager: "dnf", ServiceManager: system.ServiceManagerSystemd},
				cockpit:  system.CockpitState{Installed: true, Active: true, State: "active"},
				want:     "port 9090 check failed: cockpit port failed",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				report, err := runWithDeps(context.Background(), dependencies{
					configFilePath: func() (string, error) { return "/tmp/stackctl/config.yaml", nil },
					loadConfig:     func(string) (configpkg.Config, error) { return baseCfg, nil },
					validateConfig: func(configpkg.Config) []configpkg.ValidationIssue { return nil },
					composePath:    func(configpkg.Config) string { return "/tmp/stackctl/compose.yaml" },
					stat: func(path string) (os.FileInfo, error) {
						return fakeDoctorFileInfo{name: path, dir: !strings.HasSuffix(path, ".yaml")}, nil
					},
					platform:      func() system.Platform { return tc.platform },
					commandExists: func(string) bool { return true },
					podmanVersion: func(context.Context) (string, error) { return system.SupportedPodmanVersion, nil },
					podmanComposeVersion: func(context.Context) (string, error) {
						return system.SupportedComposeProviderVersion, nil
					},
					podmanComposeAvail: func(context.Context) bool { return true },
					openCommandName:    func() string { return "xdg-open" },
					cockpitStatus:      func(context.Context) system.CockpitState { return tc.cockpit },
					portInUse: func(port int) (bool, error) {
						if port == baseCfg.Ports.Cockpit && strings.Contains(tc.want, "port 9090 check failed") {
							return false, errors.New("cockpit port failed")
						}
						return false, nil
					},
					listContainers:  func(context.Context) ([]system.Container, error) { return nil, nil },
					redisOvercommit: func(context.Context) (system.OvercommitStatus, error) { return system.OvercommitStatus{}, nil },
				})
				if err != nil {
					t.Fatalf("runWithDeps returned error: %v", err)
				}
				if !strings.Contains(strings.Join(doctorMessages(report), "\n"), tc.want) {
					t.Fatalf("expected report to contain %q, got %+v", tc.want, report.Checks)
				}
			})
		}
	})
}

type fakeDoctorFileInfo struct {
	name string
	dir  bool
}

func (f fakeDoctorFileInfo) Name() string       { return f.name }
func (f fakeDoctorFileInfo) Size() int64        { return 0 }
func (f fakeDoctorFileInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeDoctorFileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (f fakeDoctorFileInfo) IsDir() bool        { return f.dir }
func (f fakeDoctorFileInfo) Sys() any           { return nil }
