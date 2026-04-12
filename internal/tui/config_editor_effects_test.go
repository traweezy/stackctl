package tui

import (
	"slices"
	"strings"
	"testing"

	configpkg "github.com/traweezy/stackctl/internal/config"
)

func TestSpecificFieldEffectCoversRepresentativeFieldCatalog(t *testing.T) {
	cfg := configpkg.DefaultForStack("dev-stack")
	cfg.System.PackageManager = "brew"

	testCases := []struct {
		key  string
		want string
	}{
		{key: "stack.name", want: "Renames the stack target"},
		{key: "stack.managed", want: "Switches between a stackctl-managed compose stack"},
		{key: "stack.dir", want: "Changes the stack directory"},
		{key: "stack.compose_file", want: "Changes which compose file"},
		{key: "setup.scaffold_default_stack", want: "keeps the managed compose file in sync"},
		{key: "connection.host", want: "Changes the host name"},
		{key: "setup.include_postgres", want: "Adds or removes Postgres"},
		{key: "services.postgres_container", want: "service and container name"},
		{key: "services.postgres.image", want: "image tag used by the managed stack template"},
		{key: "services.postgres.data_volume", want: "volume name used for database storage"},
		{key: "services.postgres.maintenance_database", want: "maintenance database"},
		{key: "services.postgres.max_connections", want: "max_connections"},
		{key: "services.postgres.shared_buffers", want: "shared_buffers"},
		{key: "services.postgres.log_min_duration_statement_ms", want: "slow-query duration logging threshold"},
		{key: "services.redis_container", want: "Redis service and container name"},
		{key: "services.redis.image", want: "Redis image tag"},
		{key: "services.redis.data_volume", want: "Redis volume name"},
		{key: "services.redis.appendonly", want: "appendonly persistence"},
		{key: "services.redis.save_policy", want: "snapshot save policy"},
		{key: "services.redis.maxmemory_policy", want: "eviction policy"},
		{key: "setup.include_redis", want: "Adds or removes Redis"},
		{key: "setup.include_nats", want: "Adds or removes NATS"},
		{key: "services.nats_container", want: "NATS service and container name"},
		{key: "services.nats.image", want: "NATS image tag"},
		{key: "setup.include_meilisearch", want: "Adds or removes Meilisearch"},
		{key: "services.meilisearch_container", want: "Meilisearch service and container name"},
		{key: "services.meilisearch.image", want: "Meilisearch image tag"},
		{key: "services.meilisearch.data_volume", want: "index storage"},
		{key: "setup.include_pgadmin", want: "Adds or removes pgAdmin"},
		{key: "services.pgadmin_container", want: "pgAdmin service and container name"},
		{key: "services.pgadmin.image", want: "pgAdmin image tag"},
		{key: "services.pgadmin.data_volume", want: "pgAdmin volume name"},
		{key: "services.pgadmin.server_mode", want: "server mode"},
		{key: "services.pgadmin.bootstrap_postgres_server", want: "bootstraps pgAdmin"},
		{key: "services.pgadmin.bootstrap_server_name", want: "saved pgAdmin connection name"},
		{key: "services.pgadmin.bootstrap_server_group", want: "server group"},
		{key: "ports.postgres", want: "host port published for Postgres"},
		{key: "ports.redis", want: "host port published for Redis"},
		{key: "ports.nats", want: "host port published for NATS"},
		{key: "ports.meilisearch", want: "host port published for Meilisearch"},
		{key: "ports.pgadmin", want: "host port published for pgAdmin"},
		{key: "ports.cockpit", want: "host port stackctl uses when it opens Cockpit"},
		{key: "setup.include_cockpit", want: "Cockpit"},
		{key: "connection.postgres_database", want: "default Postgres database"},
		{key: "connection.postgres_username", want: "Postgres username"},
		{key: "connection.postgres_password", want: "Postgres password"},
		{key: "connection.redis_password", want: "Redis authentication"},
		{key: "connection.redis_acl_username", want: "Redis ACL username"},
		{key: "connection.redis_acl_password", want: "Redis ACL password"},
		{key: "connection.nats_token", want: "NATS token"},
		{key: "connection.meilisearch_master_key", want: "Meilisearch master key"},
		{key: "connection.pgadmin_email", want: "pgAdmin login email"},
		{key: "connection.pgadmin_password", want: "pgAdmin login password"},
		{key: "behavior.wait_for_services_on_start", want: "start and restart wait for services"},
		{key: "behavior.startup_timeout_seconds", want: "how long stackctl waits"},
		{key: "tui.auto_refresh_interval_seconds", want: "future TUI sessions"},
		{key: "setup.install_cockpit", want: "Cockpit"},
		{key: "system.package_manager", want: "package manager"},
	}

	for _, tc := range testCases {
		spec := testConfigSpecByKey(t, tc.key)
		if got := specificFieldEffect(spec, cfg); !strings.Contains(got, tc.want) {
			t.Fatalf("specificFieldEffect(%s) = %q, want substring %q", tc.key, got, tc.want)
		}
	}

	if got := specificFieldEffect(configFieldSpec{Key: "custom.unknown"}, cfg); got != "" {
		t.Fatalf("expected unknown field effect to be blank, got %q", got)
	}
}

func TestConfigEditorEffectHelpersCoverFollowUpAndImpactBranches(t *testing.T) {
	managed := configpkg.DefaultForStack("dev-stack")
	managed.ApplyDerivedFields()

	external := managed
	external.Stack.Managed = false
	external.Setup.ScaffoldDefaultStack = false
	external.Stack.Dir = t.TempDir()
	external.Stack.ComposeFile = "compose.yaml"
	external.ApplyDerivedFields()

	noScaffold := managed
	noScaffold.Setup.ScaffoldDefaultStack = false
	noScaffold.ApplyDerivedFields()

	followUpCases := []struct {
		name string
		spec configFieldSpec
		cfg  configpkg.Config
		want string
	}{
		{name: "connection host", spec: testConfigSpecByKey(t, "connection.host"), cfg: managed, want: "helpers and dashboards only"},
		{name: "maintenance db", spec: testConfigSpecByKey(t, "services.postgres.maintenance_database"), cfg: managed, want: "future database commands only"},
		{name: "tui field", spec: testConfigSpecByKey(t, "tui.auto_refresh_interval_seconds"), cfg: managed, want: "future stackctl tui sessions only"},
		{name: "behavior field", spec: testConfigSpecByKey(t, "behavior.startup_timeout_seconds"), cfg: managed, want: "future stackctl commands only"},
		{name: "stack target", spec: testConfigSpecByKey(t, "stack.name"), cfg: managed, want: "changes which stack future stackctl commands target"},
		{name: "scaffold external", spec: testConfigSpecByKey(t, "setup.scaffold_default_stack"), cfg: external, want: "External stacks ignore managed scaffolding"},
		{name: "scaffold managed", spec: testConfigSpecByKey(t, "setup.scaffold_default_stack"), cfg: managed, want: "refresh the managed compose file"},
		{name: "external stack", spec: testConfigSpecByKey(t, "services.postgres.image"), cfg: external, want: "does not rewrite your compose file"},
		{name: "manual managed", spec: testConfigSpecByKey(t, "services.postgres.image"), cfg: noScaffold, want: "update the compose file yourself"},
		{name: "default managed", spec: testConfigSpecByKey(t, "services.postgres.image"), cfg: managed, want: "refreshes compose automatically"},
	}

	for _, tc := range followUpCases {
		if got := effectFollowUp(tc.spec, tc.cfg); !strings.Contains(got, tc.want) {
			t.Fatalf("%s: effectFollowUp() = %q, want substring %q", tc.name, got, tc.want)
		}
	}

	if got := selectedFieldEffect(configFieldSpec{Key: "custom"}, managed); !strings.Contains(got, "refreshes compose automatically") {
		t.Fatalf("expected unknown managed fields to inherit the default follow-up copy, got %q", got)
	}

	combined := selectedFieldEffect(testConfigSpecByKey(t, "connection.host"), managed)
	for _, fragment := range []string{"Changes the host name", "helpers and dashboards only"} {
		if !strings.Contains(combined, fragment) {
			t.Fatalf("expected selectedFieldEffect to contain %q, got %q", fragment, combined)
		}
	}

	boolSpec := configFieldSpec{
		Key:     "bool.toggle",
		Kind:    configFieldBool,
		GetBool: func(cfg configpkg.Config) bool { return cfg.Setup.IncludeRedis },
	}
	previous := managed
	next := managed
	next.Setup.IncludeRedis = !next.Setup.IncludeRedis
	if !configFieldChanged(boolSpec, previous, next) {
		t.Fatal("expected bool field change to be detected")
	}
	if configFieldChanged(configFieldSpec{Kind: configFieldBool}, previous, next) {
		t.Fatal("expected bool field without getter to stay unchanged")
	}

	impactCases := []struct {
		key  string
		prev configpkg.Config
		next configpkg.Config
		want configImpact
	}{
		{
			key:  "connection.host",
			prev: previous,
			next: managed,
			want: configImpact{localOnly: true},
		},
		{
			key:  "stack.name",
			prev: previous,
			next: managed,
			want: configImpact{stackTarget: true, manualFollowUp: true},
		},
		{
			key:  "stack.name",
			prev: external,
			next: external,
			want: configImpact{localOnly: true},
		},
		{
			key:  "stack.dir",
			prev: previous,
			next: managed,
			want: configImpact{stackTarget: true, manualFollowUp: true},
		},
		{
			key:  "setup.scaffold_default_stack",
			prev: previous,
			next: managed,
			want: configImpact{composeTemplate: true, manualFollowUp: true},
		},
		{
			key:  "services.postgres.image",
			prev: previous,
			next: managed,
			want: configImpact{composeTemplate: true},
		},
		{
			key:  "services.postgres.image",
			prev: previous,
			next: noScaffold,
			want: configImpact{composeTemplate: true, manualFollowUp: true},
		},
		{
			key:  "services.postgres.image",
			prev: external,
			next: external,
			want: configImpact{composeTemplate: true, localOnly: true},
		},
	}

	for _, tc := range impactCases {
		impact := &configImpact{}
		classifyConfigImpact(impact, tc.key, tc.prev, tc.next)
		if impact.localOnly != tc.want.localOnly || impact.stackTarget != tc.want.stackTarget || impact.composeTemplate != tc.want.composeTemplate || impact.manualFollowUp != tc.want.manualFollowUp {
			t.Fatalf("classifyConfigImpact(%s) = %+v, want %+v", tc.key, *impact, tc.want)
		}
	}
}

func TestConfigEditorAdditionalFormattingAndParsingBranches(t *testing.T) {
	cfg := configpkg.DefaultForStack("dev-stack")

	if got := titleCaseLabel(" "); got != "" {
		t.Fatalf("expected blank title case label, got %q", got)
	}
	if got := titleCaseLabel("x"); got != "X" {
		t.Fatalf("expected single-letter title case label, got %q", got)
	}

	boolSpec := configFieldSpec{
		Key:     "setup.include_redis",
		Kind:    configFieldBool,
		GetBool: func(cfg configpkg.Config) bool { return cfg.Setup.IncludeRedis },
	}
	if got := displayFieldValue(boolSpec, cfg, false); got != "on" {
		t.Fatalf("expected bool display value to use on/off labels, got %q", got)
	}
	if got := displayFieldValue(configFieldSpec{Kind: configFieldBool}, cfg, false); got != "(unknown)" {
		t.Fatalf("expected unknown bool display value, got %q", got)
	}

	if selectedFieldSuggestions(configFieldSpec{}, cfg) != nil {
		t.Fatal("expected missing suggestions to return nil")
	}
	packageSpec := testConfigSpecByKey(t, "system.package_manager")
	if got := selectedFieldSuggestions(packageSpec, cfg); len(got) == 0 {
		t.Fatal("expected package manager suggestions")
	}

	cfg.Services.Redis.Image = "docker.io/library/redis:8.6"
	if values := redisMaxMemoryPolicySuggestions(cfg); !slices.Contains(values, "allkeys-lrm") || !slices.Contains(values, "volatile-lrm") {
		t.Fatalf("expected redis 8.6 suggestions to include LRM policies, got %+v", values)
	}
	cfg.Services.Redis.Image = "docker.io/library/redis:8.5"
	if values := redisMaxMemoryPolicySuggestions(cfg); slices.Contains(values, "allkeys-lrm") || slices.Contains(values, "volatile-lrm") {
		t.Fatalf("did not expect redis 8.5 suggestions to include LRM policies, got %+v", values)
	}

	imageCases := []struct {
		image     string
		major     int
		minor     int
		supported bool
	}{
		{image: "docker.io/library/redis:8.6", major: 8, minor: 6, supported: true},
		{image: "redis:9", major: 9, minor: 0, supported: true},
		{image: "redis:8.6-alpine", major: 8, minor: 6, supported: true},
		{image: "redis@sha256:deadbeef", supported: false},
		{image: "redis:not-a-version", supported: false},
		{image: "", supported: false},
	}
	for _, tc := range imageCases {
		major, minor, ok := parseImageVersionTag(tc.image)
		if ok != tc.supported || major != tc.major || minor != tc.minor {
			t.Fatalf("parseImageVersionTag(%q) = (%d, %d, %v), want (%d, %d, %v)", tc.image, major, minor, ok, tc.major, tc.minor, tc.supported)
		}
	}

	if err := validPortText(cfg, "65535"); err != nil {
		t.Fatalf("expected high valid port to pass, got %v", err)
	}
	if err := validPortText(cfg, "65536"); err == nil {
		t.Fatal("expected invalid high port to fail")
	}
	if err := positiveIntText(cfg, " 42 "); err != nil {
		t.Fatalf("expected positive int text to accept 42, got %v", err)
	}
	if err := validStackNameText(cfg, "dev_stack-2"); err != nil {
		t.Fatalf("expected valid stack name to pass, got %v", err)
	}

	setNumber := intSetter(func(cfg *configpkg.Config) *int { return &cfg.Behavior.StartupTimeoutSec })
	if err := setNumber(&cfg, "12"); err != nil {
		t.Fatalf("intSetter returned error: %v", err)
	}
	if cfg.Behavior.StartupTimeoutSec != 12 {
		t.Fatalf("expected intSetter to update the target, got %d", cfg.Behavior.StartupTimeoutSec)
	}
	if err := setNumber(&cfg, "oops"); err == nil {
		t.Fatal("expected intSetter to reject non-numeric input")
	}
}
