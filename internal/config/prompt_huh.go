package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	huh "charm.land/huh/v2"
)

const (
	wizardStackModeManaged  = "managed"
	wizardStackModeExternal = "external"
)

var (
	wizardServiceLabels = map[string]string{
		"postgres":    "Postgres",
		"redis":       "Redis",
		"nats":        "NATS",
		"seaweedfs":   "SeaweedFS",
		"meilisearch": "Meilisearch",
		"pgadmin":     "pgAdmin",
	}
	packageManagerSuggestions  = []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "brew"}
	redisSavePolicySuggestions = []string{
		"3600 1 300 100 60 10000",
		"900 1 300 10",
		"300 10 60 10000",
	}
	redisMaxMemoryPolicySuggestions = []string{
		"noeviction",
		"allkeys-lru",
		"volatile-lru",
		"allkeys-random",
		"volatile-random",
		"volatile-ttl",
	}
)

type wizardState struct {
	StackName                      string
	StackMode                      string
	ExternalStackDir               string
	ExternalComposeFile            string
	AllowMissingStackDir           bool
	Services                       []string
	IncludeCockpit                 bool
	InstallCockpit                 bool
	WaitForServicesStart           bool
	StartupTimeoutSec              string
	PackageManager                 string
	PostgresContainer              string
	PostgresImage                  string
	PostgresDataVolume             string
	PostgresMaintenanceDB          string
	PostgresMaxConnections         string
	PostgresSharedBuffers          string
	PostgresLogDurationMS          string
	PostgresPort                   string
	PostgresDatabase               string
	PostgresUsername               string
	PostgresPassword               string
	RedisContainer                 string
	RedisImage                     string
	RedisDataVolume                string
	RedisAppendOnly                bool
	RedisSavePolicy                string
	RedisMaxMemoryPolicy           string
	RedisPort                      string
	RedisPassword                  string
	RedisACLUsername               string
	RedisACLPassword               string
	NATSContainer                  string
	NATSImage                      string
	NATSPort                       string
	NATSToken                      string
	SeaweedFSContainer             string
	SeaweedFSImage                 string
	SeaweedFSDataVolume            string
	SeaweedFSVolumeSizeMB          string
	SeaweedFSPort                  string
	SeaweedFSAccessKey             string
	SeaweedFSSecretKey             string
	MeilisearchContainer           string
	MeilisearchImage               string
	MeilisearchDataVolume          string
	MeilisearchPort                string
	MeilisearchMasterKey           string
	PgAdminContainer               string
	PgAdminImage                   string
	PgAdminDataVolume              string
	PgAdminServerMode              bool
	PgAdminBootstrapPostgresServer bool
	PgAdminBootstrapServerName     string
	PgAdminBootstrapServerGroup    string
	PgAdminPort                    string
	PgAdminEmail                   string
	PgAdminPassword                string
	CockpitPort                    string
}

type wizardStepID string

const (
	wizardStepStack           wizardStepID = "stack"
	wizardStepExternalStack   wizardStepID = "external-stack"
	wizardStepExternalPath    wizardStepID = "external-path"
	wizardStepServices        wizardStepID = "services"
	wizardStepPostgres        wizardStepID = "postgres"
	wizardStepRedis           wizardStepID = "redis"
	wizardStepNATS            wizardStepID = "nats"
	wizardStepSeaweedFS       wizardStepID = "seaweedfs"
	wizardStepMeilisearch     wizardStepID = "meilisearch"
	wizardStepPgAdmin         wizardStepID = "pgadmin"
	wizardStepCockpit         wizardStepID = "cockpit"
	wizardStepCockpitSettings wizardStepID = "cockpit-settings"
	wizardStepBehavior        wizardStepID = "behavior"
	wizardStepSystem          wizardStepID = "system"
	wizardStepReview          wizardStepID = "review"
)

type wizardStepSpec struct {
	ID      wizardStepID
	Label   string
	Visible func(*wizardState) bool
}

var wizardSteps = []wizardStepSpec{
	{ID: wizardStepStack, Label: "Stack basics", Visible: func(*wizardState) bool { return true }},
	{ID: wizardStepExternalStack, Label: "External stack target", Visible: func(state *wizardState) bool {
		return state.StackMode == wizardStackModeExternal
	}},
	{ID: wizardStepExternalPath, Label: "External path confirmation", Visible: func(state *wizardState) bool {
		return state.needsMissingExternalDirConfirmation()
	}},
	{ID: wizardStepServices, Label: "Service selection", Visible: func(*wizardState) bool { return true }},
	{ID: wizardStepPostgres, Label: "Postgres settings", Visible: func(state *wizardState) bool {
		return state.includesService("postgres")
	}},
	{ID: wizardStepRedis, Label: "Redis settings", Visible: func(state *wizardState) bool {
		return state.includesService("redis")
	}},
	{ID: wizardStepNATS, Label: "NATS settings", Visible: func(state *wizardState) bool {
		return state.includesService("nats")
	}},
	{ID: wizardStepSeaweedFS, Label: "SeaweedFS settings", Visible: func(state *wizardState) bool {
		return state.includesService("seaweedfs")
	}},
	{ID: wizardStepMeilisearch, Label: "Meilisearch settings", Visible: func(state *wizardState) bool {
		return state.includesService("meilisearch")
	}},
	{ID: wizardStepPgAdmin, Label: "pgAdmin settings", Visible: func(state *wizardState) bool {
		return state.includesService("pgadmin")
	}},
	{ID: wizardStepCockpit, Label: "Cockpit helpers", Visible: func(*wizardState) bool { return true }},
	{ID: wizardStepCockpitSettings, Label: "Cockpit settings", Visible: func(state *wizardState) bool {
		return state.IncludeCockpit
	}},
	{ID: wizardStepBehavior, Label: "Behavior", Visible: func(*wizardState) bool { return true }},
	{ID: wizardStepSystem, Label: "System", Visible: func(*wizardState) bool { return true }},
	{ID: wizardStepReview, Label: "Review", Visible: func(*wizardState) bool { return true }},
}

func newWizardState(cfg Config) wizardState {
	mode := wizardStackModeExternal
	if cfg.Stack.Managed {
		mode = wizardStackModeManaged
	}

	state := wizardState{
		StackName:                      cfg.Stack.Name,
		StackMode:                      mode,
		ExternalStackDir:               cfg.Stack.Dir,
		ExternalComposeFile:            cfg.Stack.ComposeFile,
		IncludeCockpit:                 cfg.Setup.IncludeCockpit,
		InstallCockpit:                 cfg.Setup.InstallCockpit,
		WaitForServicesStart:           cfg.Behavior.WaitForServicesStart,
		StartupTimeoutSec:              strconv.Itoa(cfg.Behavior.StartupTimeoutSec),
		PackageManager:                 cfg.System.PackageManager,
		PostgresContainer:              cfg.Services.PostgresContainer,
		PostgresImage:                  cfg.Services.Postgres.Image,
		PostgresDataVolume:             cfg.Services.Postgres.DataVolume,
		PostgresMaintenanceDB:          cfg.Services.Postgres.MaintenanceDatabase,
		PostgresMaxConnections:         strconv.Itoa(cfg.Services.Postgres.MaxConnections),
		PostgresSharedBuffers:          cfg.Services.Postgres.SharedBuffers,
		PostgresLogDurationMS:          strconv.Itoa(cfg.Services.Postgres.LogMinDurationStatementMS),
		PostgresPort:                   strconv.Itoa(cfg.Ports.Postgres),
		PostgresDatabase:               cfg.Connection.PostgresDatabase,
		PostgresUsername:               cfg.Connection.PostgresUsername,
		PostgresPassword:               cfg.Connection.PostgresPassword,
		RedisContainer:                 cfg.Services.RedisContainer,
		RedisImage:                     cfg.Services.Redis.Image,
		RedisDataVolume:                cfg.Services.Redis.DataVolume,
		RedisAppendOnly:                cfg.Services.Redis.AppendOnly,
		RedisSavePolicy:                cfg.Services.Redis.SavePolicy,
		RedisMaxMemoryPolicy:           cfg.Services.Redis.MaxMemoryPolicy,
		RedisPort:                      strconv.Itoa(cfg.Ports.Redis),
		RedisPassword:                  cfg.Connection.RedisPassword,
		RedisACLUsername:               cfg.Connection.RedisACLUsername,
		RedisACLPassword:               cfg.Connection.RedisACLPassword,
		NATSContainer:                  cfg.Services.NATSContainer,
		NATSImage:                      cfg.Services.NATS.Image,
		NATSPort:                       strconv.Itoa(cfg.Ports.NATS),
		NATSToken:                      cfg.Connection.NATSToken,
		SeaweedFSContainer:             cfg.Services.SeaweedFSContainer,
		SeaweedFSImage:                 cfg.Services.SeaweedFS.Image,
		SeaweedFSDataVolume:            cfg.Services.SeaweedFS.DataVolume,
		SeaweedFSVolumeSizeMB:          strconv.Itoa(cfg.Services.SeaweedFS.VolumeSizeLimitMB),
		SeaweedFSPort:                  strconv.Itoa(cfg.Ports.SeaweedFS),
		SeaweedFSAccessKey:             cfg.Connection.SeaweedFSAccessKey,
		SeaweedFSSecretKey:             cfg.Connection.SeaweedFSSecretKey,
		MeilisearchContainer:           cfg.Services.MeilisearchContainer,
		MeilisearchImage:               cfg.Services.Meilisearch.Image,
		MeilisearchDataVolume:          cfg.Services.Meilisearch.DataVolume,
		MeilisearchPort:                strconv.Itoa(cfg.Ports.Meilisearch),
		MeilisearchMasterKey:           cfg.Connection.MeilisearchMasterKey,
		PgAdminContainer:               cfg.Services.PgAdminContainer,
		PgAdminImage:                   cfg.Services.PgAdmin.Image,
		PgAdminDataVolume:              cfg.Services.PgAdmin.DataVolume,
		PgAdminServerMode:              cfg.Services.PgAdmin.ServerMode,
		PgAdminBootstrapPostgresServer: cfg.Services.PgAdmin.BootstrapPostgresServer,
		PgAdminBootstrapServerName:     cfg.Services.PgAdmin.BootstrapServerName,
		PgAdminBootstrapServerGroup:    cfg.Services.PgAdmin.BootstrapServerGroup,
		PgAdminPort:                    strconv.Itoa(cfg.Ports.PgAdmin),
		PgAdminEmail:                   cfg.Connection.PgAdminEmail,
		PgAdminPassword:                cfg.Connection.PgAdminPassword,
		CockpitPort:                    strconv.Itoa(cfg.Ports.Cockpit),
	}

	if cfg.Setup.IncludePostgres {
		state.Services = append(state.Services, "postgres")
	}
	if cfg.Setup.IncludeRedis {
		state.Services = append(state.Services, "redis")
	}
	if cfg.Setup.IncludeNATS {
		state.Services = append(state.Services, "nats")
	}
	if cfg.Setup.IncludeSeaweedFS {
		state.Services = append(state.Services, "seaweedfs")
	}
	if cfg.Setup.IncludeMeilisearch {
		state.Services = append(state.Services, "meilisearch")
	}
	if cfg.Setup.IncludePgAdmin {
		state.Services = append(state.Services, "pgadmin")
	}

	return state
}

func runHuhWizard(in io.Reader, out io.Writer, base Config) (Config, error) {
	state := newWizardState(base)

	form := buildWizardForm(&state).
		WithInput(in).
		WithOutput(out).
		WithAccessible(os.Getenv("ACCESSIBLE") != "")
	if err := form.Run(); err != nil {
		return Config{}, err
	}

	confirmed, err := runWizardReview(in, out, state)
	if err != nil {
		return Config{}, err
	}
	if !confirmed {
		return Config{}, errors.New("wizard cancelled")
	}

	cfg, err := state.toConfig(base)
	if err != nil {
		return Config{}, err
	}
	cfg.ApplyDerivedFields()

	return cfg, nil
}

func buildWizardForm(state *wizardState) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			wizardStepNote(state, wizardStepStack),
			huh.NewInput().
				Title("Stack name").
				Description("Used for `--stack`, managed resource names, and stack-specific config paths. Lowercase letters, numbers, hyphens, and underscores only.").
				Value(&state.StackName).
				Validate(validStackName),
			huh.NewSelect[string]().
				Title("Stack mode").
				Description("Managed mode scaffolds compose files under stackctl's local data dir. External mode points at an existing compose project without rewriting it.").
				Options(
					huh.NewOption("Managed stack (recommended)", wizardStackModeManaged),
					huh.NewOption("External compose stack", wizardStackModeExternal),
				).
				Value(&state.StackMode),
		).
			Title("Stack").
			Description("Choose the stack identity and how much of the compose layout stackctl should own."),
		huh.NewGroup(
			wizardStepNote(state, wizardStepExternalStack),
			huh.NewInput().
				Title("Stack directory").
				Description("Path to the external compose project. Relative paths are resolved to absolute paths before saving.").
				Value(&state.ExternalStackDir).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Compose file name").
				Description("The compose file inside that directory that stackctl should target.").
				Value(&state.ExternalComposeFile).
				Validate(nonEmpty),
		).
			Title("External Stack").
			Description("stackctl will inspect this compose project for runtime state but will not regenerate its compose files.").
			WithHideFunc(func() bool { return state.StackMode != wizardStackModeExternal }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepExternalPath),
			huh.NewConfirm().
				Title("Use the missing external directory anyway?").
				DescriptionFunc(func() string {
					absPath, err := filepath.Abs(strings.TrimSpace(state.ExternalStackDir))
					if err != nil {
						return "Choose an existing directory or explicitly confirm using a path that does not exist yet."
					}
					return fmt.Sprintf("%s does not exist yet. Confirm if you still want to save this external stack path.", absPath)
				}, &state.ExternalStackDir).
				Value(&state.AllowMissingStackDir).
				Validate(func(ok bool) error {
					if ok {
						return nil
					}
					return errors.New("choose an existing directory or confirm using the missing path")
				}),
		).
			Title("External Path").
			Description("This only appears when the chosen external stack directory does not exist yet.").
			WithHideFunc(func() bool { return !state.needsMissingExternalDirConfirmation() }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepServices),
			huh.NewMultiSelect[string]().
				Title("Services to include").
				Description("Use space to toggle services. stackctl will only scaffold, start, connect, and health-check the services selected here.").
				Options(serviceOptions(state)...).
				Height(8).
				Value(&state.Services).
				Validate(func(values []string) error {
					if len(values) == 0 {
						return errors.New("select at least one stack service")
					}
					return nil
				}),
		).
			Title("Services").
			Description("Pick the local services this stack should manage. This replaces the old one-service-at-a-time include prompts."),
		huh.NewGroup(
			wizardStepNote(state, wizardStepPostgres),
			huh.NewInput().
				Title("Postgres container name").
				Description("Used by stackctl runtime commands and the managed compose scaffold.").
				Value(&state.PostgresContainer).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Postgres image").
				Description("Container image for the managed Postgres service.").
				Value(&state.PostgresImage).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Postgres data volume").
				Description("Named volume used for persistent Postgres data.").
				Value(&state.PostgresDataVolume).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Postgres maintenance database").
				Description("Database used by admin helpers like reset and restore before switching to the app database.").
				Value(&state.PostgresMaintenanceDB).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Postgres max connections").
				Description("Passed to Postgres as max_connections for the managed local instance.").
				Value(&state.PostgresMaxConnections).
				Validate(validPositiveIntText),
			huh.NewInput().
				Title("Postgres shared buffers").
				Description("Passed to Postgres as shared_buffers. Use PostgreSQL memory syntax such as 128MB or 256MB.").
				Value(&state.PostgresSharedBuffers).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Postgres log min duration (ms)").
				Description("Set to -1 to disable query duration logging, or a positive threshold in milliseconds.").
				Value(&state.PostgresLogDurationMS).
				Validate(validPostgresLogDurationText),
			huh.NewInput().
				Title("Postgres port").
				Description("Host port exposed for Postgres and used in the DSN shown by `stackctl connect`.").
				Value(&state.PostgresPort).
				Validate(validPortText),
			huh.NewInput().
				Title("Postgres database").
				Description("Application database name used by the managed stack and DB helpers.").
				Value(&state.PostgresDatabase).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Postgres username").
				Description("Username shown in connection strings and passed to Postgres on startup.").
				Value(&state.PostgresUsername).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Postgres password").
				Description("Password used in DSNs, shell helpers, and the managed Postgres environment.").
				Value(&state.PostgresPassword).
				EchoMode(huh.EchoModePassword).
				Validate(nonEmpty),
		).
			Title("Postgres").
			Description("Configure the primary relational database service and its app-facing credentials.").
			WithHideFunc(func() bool { return !state.includesService("postgres") }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepRedis),
			huh.NewInput().
				Title("Redis container name").
				Description("Used by stackctl runtime, exec, and log commands.").
				Value(&state.RedisContainer).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Redis image").
				Description("Container image for the managed Redis service.").
				Value(&state.RedisImage).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Redis data volume").
				Description("Named volume for Redis persistence when snapshots or append-only mode are enabled.").
				Value(&state.RedisDataVolume).
				Validate(nonEmpty),
			huh.NewConfirm().
				Title("Enable Redis append-only persistence").
				Description("Turn this on when you want durable local writes instead of a purely ephemeral cache.").
				Value(&state.RedisAppendOnly),
			huh.NewInput().
				Title("Redis save policy").
				Description("Snapshot policy passed to Redis. Leave the default if you just want sensible local durability.").
				Suggestions(redisSavePolicySuggestions).
				Value(&state.RedisSavePolicy).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Redis maxmemory policy").
				Description("Eviction policy to apply when Redis hits its memory ceiling.").
				Suggestions(redisMaxMemoryPolicySuggestions).
				Value(&state.RedisMaxMemoryPolicy).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Redis port").
				Description("Host port exposed for Redis and shown in stackctl connection helpers.").
				Value(&state.RedisPort).
				Validate(validPortText),
			huh.NewInput().
				Title("Redis password").
				Description("Optional. Leave blank to keep Redis auth disabled.").
				Value(&state.RedisPassword).
				EchoMode(huh.EchoModePassword),
			huh.NewInput().
				Title("Redis ACL username").
				Description("Optional. Set this and the ACL password to bootstrap a named Redis ACL user for app connections.").
				Value(&state.RedisACLUsername),
			huh.NewInput().
				Title("Redis ACL password").
				Description("Optional. Leave blank unless you also set a Redis ACL username.").
				Value(&state.RedisACLPassword).
				EchoMode(huh.EchoModePassword),
		).
			Title("Redis").
			Description("Tune local cache behavior and optional authentication for Redis.").
			WithHideFunc(func() bool { return !state.includesService("redis") }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepNATS),
			huh.NewInput().
				Title("NATS container name").
				Description("Used by stackctl runtime, logs, and exec helpers.").
				Value(&state.NATSContainer).
				Validate(nonEmpty),
			huh.NewInput().
				Title("NATS image").
				Description("Container image for the managed NATS service.").
				Value(&state.NATSImage).
				Validate(nonEmpty),
			huh.NewInput().
				Title("NATS port").
				Description("Host port exposed for the NATS client endpoint.").
				Value(&state.NATSPort).
				Validate(validPortText),
			huh.NewInput().
				Title("NATS auth token").
				Description("Used in the managed `nats.conf`, shown in the NATS DSN, and available through copy helpers.").
				Value(&state.NATSToken).
				EchoMode(huh.EchoModePassword).
				Validate(nonEmpty),
		).
			Title("NATS").
			Description("Configure the lightweight messaging service and its client token.").
			WithHideFunc(func() bool { return !state.includesService("nats") }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepSeaweedFS),
			huh.NewInput().
				Title("SeaweedFS container name").
				Description("Used by stackctl runtime, logs, and exec helpers.").
				Value(&state.SeaweedFSContainer).
				Validate(nonEmpty),
			huh.NewInput().
				Title("SeaweedFS image").
				Description("Container image for the managed SeaweedFS S3 service.").
				Value(&state.SeaweedFSImage).
				Validate(nonEmpty),
			huh.NewInput().
				Title("SeaweedFS data volume").
				Description("Named volume for SeaweedFS filer and object data.").
				Value(&state.SeaweedFSDataVolume).
				Validate(nonEmpty),
			huh.NewInput().
				Title("SeaweedFS volume size limit (MB)").
				Description("Per-volume growth cap for local development. Keep this modest unless you need large buckets locally.").
				Value(&state.SeaweedFSVolumeSizeMB).
				Validate(validPositiveIntText),
			huh.NewInput().
				Title("SeaweedFS S3 port").
				Description("Host port exposed for the S3-compatible endpoint.").
				Value(&state.SeaweedFSPort).
				Validate(validPortText),
			huh.NewInput().
				Title("SeaweedFS access key").
				Description("S3 access key shown in copy helpers and passed into the managed container environment.").
				Value(&state.SeaweedFSAccessKey).
				Validate(nonEmpty),
			huh.NewInput().
				Title("SeaweedFS secret key").
				Description("S3 secret key used for the managed SeaweedFS endpoint.").
				Value(&state.SeaweedFSSecretKey).
				EchoMode(huh.EchoModePassword).
				Validate(nonEmpty),
		).
			Title("SeaweedFS").
			Description("Configure optional S3-compatible object storage with a single-container local SeaweedFS setup.").
			WithHideFunc(func() bool { return !state.includesService("seaweedfs") }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepMeilisearch),
			huh.NewInput().
				Title("Meilisearch container name").
				Description("Used by stackctl runtime, logs, and exec helpers.").
				Value(&state.MeilisearchContainer).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Meilisearch image").
				Description("Container image for the managed Meilisearch service.").
				Value(&state.MeilisearchImage).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Meilisearch data volume").
				Description("Named volume for Meilisearch database files.").
				Value(&state.MeilisearchDataVolume).
				Validate(nonEmpty),
			huh.NewInput().
				Title("Meilisearch port").
				Description("Host port exposed for the Meilisearch API and preview interface.").
				Value(&state.MeilisearchPort).
				Validate(validPortText),
			huh.NewInput().
				Title("Meilisearch master key").
				Description("Used to protect the Meilisearch instance and exported as the local-dev API key helper.").
				Value(&state.MeilisearchMasterKey).
				EchoMode(huh.EchoModePassword).
				Validate(minLen(16)),
		).
			Title("Meilisearch").
			Description("Configure optional local search with a protected Meilisearch instance and preview UI.").
			WithHideFunc(func() bool { return !state.includesService("meilisearch") }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepPgAdmin),
			huh.NewInput().
				Title("pgAdmin container name").
				Description("Used by stackctl runtime inspection and log helpers.").
				Value(&state.PgAdminContainer).
				Validate(nonEmpty),
			huh.NewInput().
				Title("pgAdmin image").
				Description("Container image for the managed pgAdmin service.").
				Value(&state.PgAdminImage).
				Validate(nonEmpty),
			huh.NewInput().
				Title("pgAdmin data volume").
				Description("Named volume for pgAdmin state and saved connections.").
				Value(&state.PgAdminDataVolume).
				Validate(nonEmpty),
			huh.NewConfirm().
				Title("Run pgAdmin in server mode").
				Description("Enable this when you want pgAdmin's multi-server behavior instead of the lighter desktop-style mode.").
				Value(&state.PgAdminServerMode),
			huh.NewConfirm().
				Title("Bootstrap pgAdmin with the local Postgres server").
				Description("Preload the managed Postgres instance into pgAdmin using a generated servers.json and pgpass file.").
				Value(&state.PgAdminBootstrapPostgresServer),
			huh.NewInput().
				Title("pgAdmin bootstrap server name").
				Description("The saved pgAdmin connection name for the managed Postgres instance.").
				Value(&state.PgAdminBootstrapServerName).
				Validate(nonEmpty),
			huh.NewInput().
				Title("pgAdmin bootstrap server group").
				Description("The pgAdmin server group used for the managed Postgres bootstrap entry.").
				Value(&state.PgAdminBootstrapServerGroup).
				Validate(nonEmpty),
			huh.NewInput().
				Title("pgAdmin port").
				Description("Host port exposed for the pgAdmin web UI.").
				Value(&state.PgAdminPort).
				Validate(validPortText),
			huh.NewInput().
				Title("pgAdmin email").
				Description("Login email for the managed pgAdmin instance.").
				Value(&state.PgAdminEmail).
				Validate(nonEmpty),
			huh.NewInput().
				Title("pgAdmin password").
				Description("Login password for the managed pgAdmin instance.").
				Value(&state.PgAdminPassword).
				EchoMode(huh.EchoModePassword).
				Validate(nonEmpty),
		).
			Title("pgAdmin").
			Description("Configure the optional Postgres web UI that ships with the managed stack.").
			WithHideFunc(func() bool { return !state.includesService("pgadmin") }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepCockpit),
			huh.NewConfirm().
				Title("Include Cockpit helpers").
				Description("Cockpit is managed outside the compose stack. Enable this if you want helper output, open actions, and optional setup-time installation.").
				Value(&state.IncludeCockpit),
		).
			Title("Cockpit").
			Description("Configure optional host-level integration separately from stack-managed services."),
		huh.NewGroup(
			wizardStepNote(state, wizardStepCockpitSettings),
			huh.NewInput().
				Title("Cockpit port").
				Description("Used for Cockpit URLs and health hints when Cockpit helpers are enabled.").
				Value(&state.CockpitPort).
				Validate(validPortText),
			huh.NewConfirm().
				Title("Install Cockpit during setup").
				Description("If enabled, `stackctl setup --install` will try to install cockpit and cockpit-podman when supported.").
				Value(&state.InstallCockpit),
		).
			Title("Cockpit Settings").
			Description("These settings only matter when Cockpit helpers are enabled.").
			WithHideFunc(func() bool { return !state.IncludeCockpit }),
		huh.NewGroup(
			wizardStepNote(state, wizardStepBehavior),
			huh.NewConfirm().
				Title("Wait for selected services on start").
				Description("When enabled, stackctl waits for service ports to come up before returning from start and restart.").
				Value(&state.WaitForServicesStart),
			huh.NewInput().
				Title("Startup timeout in seconds").
				Description("Maximum time stackctl should wait for readiness before start or restart fails.").
				Value(&state.StartupTimeoutSec).
				Validate(validPositiveIntText),
		).
			Title("Behavior").
			Description("Choose how stack lifecycle commands should behave after launch."),
		huh.NewGroup(
			wizardStepNote(state, wizardStepSystem),
			huh.NewInput().
				Title("Package manager").
				Description("Used by `stackctl setup --install` for supported dependency installation on the local machine.").
				Suggestions(packageManagerSuggestions).
				Value(&state.PackageManager).
				Validate(nonEmpty),
		).
			Title("System").
			Description("Tell stackctl how to install supported packages when you opt into automatic setup."),
	)
}

func runWizardReview(in io.Reader, out io.Writer, state wizardState) (bool, error) {
	confirmed := true
	reviewForm := huh.NewForm(
		huh.NewGroup(
			wizardStepNote(&state, wizardStepReview),
			huh.NewNote().
				Title("Review").
				Description(state.reviewSummary()).
				Height(16),
			huh.NewConfirm().
				Title("Save this configuration?").
				Description("Select No to abort without changing the config file.").
				Affirmative("Save").
				Negative("Cancel").
				Value(&confirmed),
		).
			Title("Review").
			Description("Check the selected services, stack mode, and key ports before saving."),
	).
		WithInput(in).
		WithOutput(out).
		WithAccessible(os.Getenv("ACCESSIBLE") != "")

	if err := reviewForm.Run(); err != nil {
		return false, err
	}

	return confirmed, nil
}

func wizardStepNote(state *wizardState, step wizardStepID) *huh.Note {
	return huh.NewNote().
		TitleFunc(func() string {
			position, total := wizardStepPosition(state, step)
			label := wizardStepLabel(step)
			if position == 0 || total == 0 {
				return label
			}
			return fmt.Sprintf("Step %d of %d  •  %s", position, total, label)
		}, state).
		DescriptionFunc(func() string {
			next := wizardNextStepLabel(state, step)
			if next == "" {
				return "Final confirmation before the config is written."
			}
			return "Next: " + next
		}, state)
}

func wizardStepPosition(state *wizardState, target wizardStepID) (int, int) {
	visible := visibleWizardSteps(state)
	for idx, step := range visible {
		if step.ID == target {
			return idx + 1, len(visible)
		}
	}
	return 0, len(visible)
}

func visibleWizardSteps(state *wizardState) []wizardStepSpec {
	visible := make([]wizardStepSpec, 0, len(wizardSteps))
	for _, step := range wizardSteps {
		if step.Visible != nil && !step.Visible(state) {
			continue
		}
		visible = append(visible, step)
	}
	return visible
}

func wizardStepLabel(target wizardStepID) string {
	for _, step := range wizardSteps {
		if step.ID == target {
			return step.Label
		}
	}
	return "Setup"
}

func wizardNextStepLabel(state *wizardState, target wizardStepID) string {
	visible := visibleWizardSteps(state)
	for idx, step := range visible {
		if step.ID != target {
			continue
		}
		if idx+1 >= len(visible) {
			return ""
		}
		return visible[idx+1].Label
	}
	return ""
}

func (s wizardState) includesService(name string) bool {
	return slices.Contains(s.Services, name)
}

func (s wizardState) needsMissingExternalDirConfirmation() bool {
	if s.StackMode != wizardStackModeExternal {
		return false
	}

	path := strings.TrimSpace(s.ExternalStackDir)
	if path == "" {
		return false
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(absPath)
	return err != nil || !info.IsDir()
}

func (s wizardState) toConfig(base Config) (Config, error) {
	cfg := base

	cfg.Stack.Name = strings.TrimSpace(s.StackName)
	switch s.StackMode {
	case wizardStackModeManaged:
		stackDir, err := ManagedStackDir(cfg.Stack.Name)
		if err != nil {
			return Config{}, err
		}
		cfg.Stack.Managed = true
		cfg.Stack.Dir = stackDir
		cfg.Stack.ComposeFile = DefaultComposeFileName
		cfg.Setup.ScaffoldDefaultStack = true
	case wizardStackModeExternal:
		stackDir, err := filepath.Abs(strings.TrimSpace(s.ExternalStackDir))
		if err != nil {
			return Config{}, fmt.Errorf("resolve stack directory %q: %w", s.ExternalStackDir, err)
		}
		cfg.Stack.Managed = false
		cfg.Stack.Dir = stackDir
		cfg.Stack.ComposeFile = strings.TrimSpace(s.ExternalComposeFile)
		cfg.Setup.ScaffoldDefaultStack = false
	default:
		return Config{}, fmt.Errorf("invalid stack mode %q", s.StackMode)
	}

	cfg.Setup.IncludePostgres = s.includesService("postgres")
	cfg.Setup.IncludeRedis = s.includesService("redis")
	cfg.Setup.IncludeNATS = s.includesService("nats")
	cfg.Setup.IncludeSeaweedFS = s.includesService("seaweedfs")
	cfg.Setup.IncludeMeilisearch = s.includesService("meilisearch")
	cfg.Setup.IncludePgAdmin = s.includesService("pgadmin")
	cfg.Setup.IncludeCockpit = s.IncludeCockpit
	cfg.Setup.InstallCockpit = s.InstallCockpit

	cfg.Behavior.WaitForServicesStart = s.WaitForServicesStart

	startupTimeout, err := parsePositiveInt(s.StartupTimeoutSec)
	if err != nil {
		return Config{}, fmt.Errorf("startup timeout: %w", err)
	}
	cfg.Behavior.StartupTimeoutSec = startupTimeout
	cfg.System.PackageManager = strings.TrimSpace(s.PackageManager)

	cfg.Services.PostgresContainer = strings.TrimSpace(s.PostgresContainer)
	cfg.Services.Postgres.Image = strings.TrimSpace(s.PostgresImage)
	cfg.Services.Postgres.DataVolume = strings.TrimSpace(s.PostgresDataVolume)
	cfg.Services.Postgres.MaintenanceDatabase = strings.TrimSpace(s.PostgresMaintenanceDB)
	cfg.Services.Postgres.MaxConnections, err = parsePositiveInt(s.PostgresMaxConnections)
	if err != nil {
		return Config{}, fmt.Errorf("postgres max connections: %w", err)
	}
	cfg.Services.Postgres.SharedBuffers = strings.TrimSpace(s.PostgresSharedBuffers)
	cfg.Services.Postgres.LogMinDurationStatementMS, err = parsePostgresLogDurationMS(s.PostgresLogDurationMS)
	if err != nil {
		return Config{}, fmt.Errorf("postgres log min duration: %w", err)
	}
	cfg.Connection.PostgresDatabase = strings.TrimSpace(s.PostgresDatabase)
	cfg.Connection.PostgresUsername = strings.TrimSpace(s.PostgresUsername)
	cfg.Connection.PostgresPassword = strings.TrimSpace(s.PostgresPassword)
	cfg.Ports.Postgres, err = parsePort(s.PostgresPort)
	if err != nil {
		return Config{}, fmt.Errorf("postgres port: %w", err)
	}

	cfg.Services.RedisContainer = strings.TrimSpace(s.RedisContainer)
	cfg.Services.Redis.Image = strings.TrimSpace(s.RedisImage)
	cfg.Services.Redis.DataVolume = strings.TrimSpace(s.RedisDataVolume)
	cfg.Services.Redis.AppendOnly = s.RedisAppendOnly
	cfg.Services.Redis.SavePolicy = strings.TrimSpace(s.RedisSavePolicy)
	cfg.Services.Redis.MaxMemoryPolicy = strings.TrimSpace(s.RedisMaxMemoryPolicy)
	cfg.Connection.RedisPassword = strings.TrimSpace(s.RedisPassword)
	cfg.Connection.RedisACLUsername = strings.TrimSpace(s.RedisACLUsername)
	cfg.Connection.RedisACLPassword = strings.TrimSpace(s.RedisACLPassword)
	cfg.Ports.Redis, err = parsePort(s.RedisPort)
	if err != nil {
		return Config{}, fmt.Errorf("redis port: %w", err)
	}

	cfg.Services.NATSContainer = strings.TrimSpace(s.NATSContainer)
	cfg.Services.NATS.Image = strings.TrimSpace(s.NATSImage)
	cfg.Connection.NATSToken = strings.TrimSpace(s.NATSToken)
	cfg.Ports.NATS, err = parsePort(s.NATSPort)
	if err != nil {
		return Config{}, fmt.Errorf("nats port: %w", err)
	}

	cfg.Services.SeaweedFSContainer = strings.TrimSpace(s.SeaweedFSContainer)
	cfg.Services.SeaweedFS.Image = strings.TrimSpace(s.SeaweedFSImage)
	cfg.Services.SeaweedFS.DataVolume = strings.TrimSpace(s.SeaweedFSDataVolume)
	cfg.Services.SeaweedFS.VolumeSizeLimitMB, err = parsePositiveInt(s.SeaweedFSVolumeSizeMB)
	if err != nil {
		return Config{}, fmt.Errorf("seaweedfs volume size limit: %w", err)
	}
	cfg.Connection.SeaweedFSAccessKey = strings.TrimSpace(s.SeaweedFSAccessKey)
	cfg.Connection.SeaweedFSSecretKey = strings.TrimSpace(s.SeaweedFSSecretKey)
	cfg.Ports.SeaweedFS, err = parsePort(s.SeaweedFSPort)
	if err != nil {
		return Config{}, fmt.Errorf("seaweedfs port: %w", err)
	}

	cfg.Services.MeilisearchContainer = strings.TrimSpace(s.MeilisearchContainer)
	cfg.Services.Meilisearch.Image = strings.TrimSpace(s.MeilisearchImage)
	cfg.Services.Meilisearch.DataVolume = strings.TrimSpace(s.MeilisearchDataVolume)
	cfg.Connection.MeilisearchMasterKey = strings.TrimSpace(s.MeilisearchMasterKey)
	cfg.Ports.Meilisearch, err = parsePort(s.MeilisearchPort)
	if err != nil {
		return Config{}, fmt.Errorf("meilisearch port: %w", err)
	}

	cfg.Services.PgAdminContainer = strings.TrimSpace(s.PgAdminContainer)
	cfg.Services.PgAdmin.Image = strings.TrimSpace(s.PgAdminImage)
	cfg.Services.PgAdmin.DataVolume = strings.TrimSpace(s.PgAdminDataVolume)
	cfg.Services.PgAdmin.ServerMode = s.PgAdminServerMode
	cfg.Services.PgAdmin.BootstrapPostgresServer = s.PgAdminBootstrapPostgresServer && cfg.Setup.IncludePostgres
	cfg.Services.PgAdmin.BootstrapServerName = strings.TrimSpace(s.PgAdminBootstrapServerName)
	cfg.Services.PgAdmin.BootstrapServerGroup = strings.TrimSpace(s.PgAdminBootstrapServerGroup)
	cfg.Connection.PgAdminEmail = strings.TrimSpace(s.PgAdminEmail)
	cfg.Connection.PgAdminPassword = strings.TrimSpace(s.PgAdminPassword)
	cfg.Ports.PgAdmin, err = parsePort(s.PgAdminPort)
	if err != nil {
		return Config{}, fmt.Errorf("pgadmin port: %w", err)
	}

	cfg.Ports.Cockpit, err = parsePort(s.CockpitPort)
	if err != nil {
		return Config{}, fmt.Errorf("cockpit port: %w", err)
	}

	return cfg, nil
}

func (s wizardState) reviewSummary() string {
	lines := []string{
		fmt.Sprintf("Stack: %s", s.StackName),
		fmt.Sprintf("Mode: %s", s.stackModeLabel()),
		fmt.Sprintf("Services: %s", strings.Join(s.serviceDisplayNames(), ", ")),
		fmt.Sprintf("Cockpit helpers: %s", enabledDisabledLabel(s.IncludeCockpit)),
		"",
		"Ports:",
	}

	if s.includesService("postgres") {
		lines = append(lines, fmt.Sprintf("  - Postgres: %s", s.PostgresPort))
	}
	if s.includesService("redis") {
		lines = append(lines, fmt.Sprintf("  - Redis: %s", s.RedisPort))
	}
	if s.includesService("nats") {
		lines = append(lines, fmt.Sprintf("  - NATS: %s", s.NATSPort))
	}
	if s.includesService("seaweedfs") {
		lines = append(lines, fmt.Sprintf("  - SeaweedFS: %s", s.SeaweedFSPort))
	}
	if s.includesService("meilisearch") {
		lines = append(lines, fmt.Sprintf("  - Meilisearch: %s", s.MeilisearchPort))
	}
	if s.includesService("pgadmin") {
		lines = append(lines, fmt.Sprintf("  - pgAdmin: %s", s.PgAdminPort))
	}
	if s.IncludeCockpit {
		lines = append(lines, fmt.Sprintf("  - Cockpit: %s", s.CockpitPort))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Wait for services: %s", enabledDisabledLabel(s.WaitForServicesStart)))
	lines = append(lines, fmt.Sprintf("Startup timeout: %ss", s.StartupTimeoutSec))
	lines = append(lines, fmt.Sprintf("Package manager: %s", s.PackageManager))

	if s.StackMode == wizardStackModeManaged {
		if stackDir, err := ManagedStackDir(s.StackName); err == nil {
			lines = append(lines, fmt.Sprintf("Managed stack dir: %s", stackDir))
		}
	} else {
		lines = append(lines, fmt.Sprintf("External stack dir: %s", s.ExternalStackDir))
		lines = append(lines, fmt.Sprintf("Compose file: %s", s.ExternalComposeFile))
	}

	return strings.Join(lines, "\n")
}

func (s wizardState) stackModeLabel() string {
	if s.StackMode == wizardStackModeManaged {
		return "Managed"
	}
	return "External"
}

func (s wizardState) serviceDisplayNames() []string {
	services := make([]string, 0, len(s.Services))
	for _, service := range s.Services {
		label, ok := wizardServiceLabels[service]
		if !ok {
			label = service
		}
		services = append(services, label)
	}
	return services
}

func serviceOptions(state *wizardState) []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Postgres", "postgres").Selected(state.includesService("postgres")),
		huh.NewOption("Redis", "redis").Selected(state.includesService("redis")),
		huh.NewOption("NATS", "nats").Selected(state.includesService("nats")),
		huh.NewOption("SeaweedFS (S3)", "seaweedfs").Selected(state.includesService("seaweedfs")),
		huh.NewOption("Meilisearch", "meilisearch").Selected(state.includesService("meilisearch")),
		huh.NewOption("pgAdmin", "pgadmin").Selected(state.includesService("pgadmin")),
	}
}

func parsePort(value string) (int, error) {
	parsed, err := parsePositiveInt(value)
	if err != nil {
		return 0, err
	}
	if err := validPort(parsed); err != nil {
		return 0, err
	}
	return parsed, nil
}

func parsePositiveInt(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if err := nonEmpty(trimmed); err != nil {
		return 0, err
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("enter a valid number")
	}
	if err := positiveInt(parsed); err != nil {
		return 0, err
	}

	return parsed, nil
}

func parsePostgresLogDurationMS(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if err := nonEmpty(trimmed); err != nil {
		return 0, err
	}
	if trimmed == "-1" {
		return -1, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("enter -1 or a positive number")
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("enter -1 or a positive number")
	}
	return parsed, nil
}

func validPortText(value string) error {
	_, err := parsePort(value)
	return err
}

func validPositiveIntText(value string) error {
	_, err := parsePositiveInt(value)
	return err
}

func validPostgresLogDurationText(value string) error {
	_, err := parsePostgresLogDurationMS(value)
	return err
}

func enabledDisabledLabel(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}
