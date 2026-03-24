package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"
)

const maxIntValue = int(^uint(0) >> 1)

func RunWizard(in io.Reader, out io.Writer, base Config) (Config, error) {
	if shouldUsePlainWizard(in, out) {
		return runPlainWizard(in, out, base)
	}

	return runHuhWizard(in, out, base)
}

func runPlainWizard(in io.Reader, out io.Writer, base Config) (Config, error) {
	session := promptSession{
		reader: bufio.NewReader(in),
		out:    out,
	}

	cfg := base

	stackName, err := session.askString("Stack name", cfg.Stack.Name, validStackName)
	if err != nil {
		return Config{}, err
	}
	cfg.Stack.Name = stackName

	managedDefaultDir, err := ManagedStackDir(cfg.Stack.Name)
	if err != nil {
		return Config{}, err
	}

	manageStack, err := session.askBool(
		fmt.Sprintf("Create and manage the default stack in %s", managedDefaultDir),
		cfg.Stack.Managed && cfg.Setup.ScaffoldDefaultStack,
	)
	if err != nil {
		return Config{}, err
	}

	if manageStack {
		cfg.Stack.Managed = true
		cfg.Stack.Dir = managedDefaultDir
		cfg.Stack.ComposeFile = DefaultComposeFileName
		cfg.Setup.ScaffoldDefaultStack = true
	} else {
		stackDir, err := session.askStackDir(cfg.Stack.Dir)
		if err != nil {
			return Config{}, err
		}
		cfg.Stack.Dir = stackDir

		composeFile, err := session.askString("Compose file name", cfg.Stack.ComposeFile, nonEmpty)
		if err != nil {
			return Config{}, err
		}
		cfg.Stack.ComposeFile = composeFile
		cfg.Stack.Managed = false
		cfg.Setup.ScaffoldDefaultStack = false
	}

	if err := session.configurePostgres(&cfg); err != nil {
		return Config{}, err
	}
	if err := session.configureRedis(&cfg); err != nil {
		return Config{}, err
	}
	if err := session.configureNATS(&cfg); err != nil {
		return Config{}, err
	}
	if err := session.configurePgAdmin(&cfg); err != nil {
		return Config{}, err
	}

	if cfg.EnabledStackServiceCount() == 0 {
		return Config{}, fmt.Errorf("at least one stack service must be enabled")
	}

	if err := session.configureCockpit(&cfg); err != nil {
		return Config{}, err
	}

	if err := session.printSection("Behavior"); err != nil {
		return Config{}, err
	}

	waitForServices, err := session.askBool("Wait for services on start", cfg.Behavior.WaitForServicesStart)
	if err != nil {
		return Config{}, err
	}
	cfg.Behavior.WaitForServicesStart = waitForServices

	timeoutSeconds, err := session.askInt("Startup timeout in seconds", cfg.Behavior.StartupTimeoutSec, positiveInt)
	if err != nil {
		return Config{}, err
	}
	cfg.Behavior.StartupTimeoutSec = timeoutSeconds

	if !cfg.Stack.Managed {
		cfg.Setup.ScaffoldDefaultStack = false
	}

	if err := session.printSection("System"); err != nil {
		return Config{}, err
	}

	packageManager, err := session.askString("Package manager", cfg.System.PackageManager, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.System.PackageManager = packageManager

	cfg.ApplyDerivedFields()

	return cfg, nil
}

func shouldUsePlainWizard(in io.Reader, out io.Writer) bool {
	if os.Getenv("STACKCTL_WIZARD_PLAIN") != "" {
		return true
	}

	inputFile, inputOK := in.(*os.File)
	outputFile, outputOK := out.(*os.File)
	if !inputOK || !outputOK {
		return true
	}

	inputFD, ok := terminalFD(inputFile)
	if !ok {
		return true
	}
	outputFD, ok := terminalFD(outputFile)
	if !ok {
		return true
	}

	return !term.IsTerminal(inputFD) || !term.IsTerminal(outputFD)
}

func terminalFD(file *os.File) (int, bool) {
	fd := file.Fd()
	if fd > uintptr(maxIntValue) {
		return 0, false
	}

	return int(fd), true
}

func PromptYesNo(in io.Reader, out io.Writer, question string, defaultYes bool) (bool, error) {
	session := promptSession{
		reader: bufio.NewReader(in),
		out:    out,
	}

	return session.askBool(question, defaultYes)
}

type promptSession struct {
	reader *bufio.Reader
	out    io.Writer
}

func (p promptSession) printSection(title string) error {
	_, err := fmt.Fprintf(p.out, "\n[%s]\n", title)
	return err
}

func (p promptSession) configurePostgres(cfg *Config) error {
	includePostgres, err := p.askBool("Include Postgres in the stack", cfg.Setup.IncludePostgres)
	if err != nil {
		return err
	}
	cfg.Setup.IncludePostgres = includePostgres
	if !cfg.Setup.IncludePostgres {
		return nil
	}

	if err := p.printSection("Postgres"); err != nil {
		return err
	}

	postgresContainer, err := p.askString("Postgres container name", cfg.Services.PostgresContainer, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.PostgresContainer = postgresContainer

	postgresImage, err := p.askString("Postgres image", cfg.Services.Postgres.Image, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.Postgres.Image = postgresImage

	postgresDataVolume, err := p.askString("Postgres data volume", cfg.Services.Postgres.DataVolume, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.Postgres.DataVolume = postgresDataVolume

	maintenanceDatabase, err := p.askString("Postgres maintenance database", cfg.Services.Postgres.MaintenanceDatabase, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.Postgres.MaintenanceDatabase = maintenanceDatabase

	postgresPort, err := p.askPort("Postgres port", cfg.Ports.Postgres)
	if err != nil {
		return err
	}
	cfg.Ports.Postgres = postgresPort

	postgresDatabase, err := p.askString("Postgres database name", cfg.Connection.PostgresDatabase, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Connection.PostgresDatabase = postgresDatabase

	postgresUsername, err := p.askString("Postgres username", cfg.Connection.PostgresUsername, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Connection.PostgresUsername = postgresUsername

	postgresPassword, err := p.askString("Postgres password", cfg.Connection.PostgresPassword, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Connection.PostgresPassword = postgresPassword

	return nil
}

func (p promptSession) configureRedis(cfg *Config) error {
	includeRedis, err := p.askBool("Include Redis in the stack", cfg.Setup.IncludeRedis)
	if err != nil {
		return err
	}
	cfg.Setup.IncludeRedis = includeRedis
	if !cfg.Setup.IncludeRedis {
		return nil
	}

	if err := p.printSection("Redis"); err != nil {
		return err
	}

	redisContainer, err := p.askString("Redis container name", cfg.Services.RedisContainer, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.RedisContainer = redisContainer

	redisImage, err := p.askString("Redis image", cfg.Services.Redis.Image, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.Redis.Image = redisImage

	redisDataVolume, err := p.askString("Redis data volume", cfg.Services.Redis.DataVolume, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.Redis.DataVolume = redisDataVolume

	redisAppendOnly, err := p.askBool("Enable Redis appendonly persistence", cfg.Services.Redis.AppendOnly)
	if err != nil {
		return err
	}
	cfg.Services.Redis.AppendOnly = redisAppendOnly

	redisSavePolicy, err := p.askString("Redis save policy", cfg.Services.Redis.SavePolicy, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.Redis.SavePolicy = redisSavePolicy

	redisMaxMemoryPolicy, err := p.askString("Redis maxmemory policy", cfg.Services.Redis.MaxMemoryPolicy, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.Redis.MaxMemoryPolicy = redisMaxMemoryPolicy

	redisPort, err := p.askPort("Redis port", cfg.Ports.Redis)
	if err != nil {
		return err
	}
	cfg.Ports.Redis = redisPort

	redisPassword, err := p.askString("Redis password (leave blank to disable auth)", cfg.Connection.RedisPassword, nil)
	if err != nil {
		return err
	}
	cfg.Connection.RedisPassword = redisPassword

	return nil
}

func (p promptSession) configureNATS(cfg *Config) error {
	includeNATS, err := p.askBool("Include NATS in the stack", cfg.Setup.IncludeNATS)
	if err != nil {
		return err
	}
	cfg.Setup.IncludeNATS = includeNATS
	if !cfg.Setup.IncludeNATS {
		return nil
	}

	if err := p.printSection("NATS"); err != nil {
		return err
	}

	natsContainer, err := p.askString("NATS container name", cfg.Services.NATSContainer, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.NATSContainer = natsContainer

	natsImage, err := p.askString("NATS image", cfg.Services.NATS.Image, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.NATS.Image = natsImage

	natsPort, err := p.askPort("NATS port", cfg.Ports.NATS)
	if err != nil {
		return err
	}
	cfg.Ports.NATS = natsPort

	natsToken, err := p.askString("NATS auth token", cfg.Connection.NATSToken, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Connection.NATSToken = natsToken

	return nil
}

func (p promptSession) configurePgAdmin(cfg *Config) error {
	includePgAdmin, err := p.askBool("Include pgAdmin in the stack", cfg.Setup.IncludePgAdmin)
	if err != nil {
		return err
	}
	cfg.Setup.IncludePgAdmin = includePgAdmin
	if !cfg.Setup.IncludePgAdmin {
		return nil
	}

	if err := p.printSection("pgAdmin"); err != nil {
		return err
	}

	pgAdminContainer, err := p.askString("pgAdmin container name", cfg.Services.PgAdminContainer, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.PgAdminContainer = pgAdminContainer

	pgAdminImage, err := p.askString("pgAdmin image", cfg.Services.PgAdmin.Image, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.PgAdmin.Image = pgAdminImage

	pgAdminDataVolume, err := p.askString("pgAdmin data volume", cfg.Services.PgAdmin.DataVolume, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Services.PgAdmin.DataVolume = pgAdminDataVolume

	pgAdminServerMode, err := p.askBool("Run pgAdmin in server mode", cfg.Services.PgAdmin.ServerMode)
	if err != nil {
		return err
	}
	cfg.Services.PgAdmin.ServerMode = pgAdminServerMode

	pgAdminPort, err := p.askPort("pgAdmin port", cfg.Ports.PgAdmin)
	if err != nil {
		return err
	}
	cfg.Ports.PgAdmin = pgAdminPort

	pgAdminEmail, err := p.askString("pgAdmin email", cfg.Connection.PgAdminEmail, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Connection.PgAdminEmail = pgAdminEmail

	pgAdminPassword, err := p.askString("pgAdmin password", cfg.Connection.PgAdminPassword, nonEmpty)
	if err != nil {
		return err
	}
	cfg.Connection.PgAdminPassword = pgAdminPassword

	return nil
}

func (p promptSession) configureCockpit(cfg *Config) error {
	includeCockpit, err := p.askBool("Include Cockpit helpers", cfg.Setup.IncludeCockpit)
	if err != nil {
		return err
	}
	cfg.Setup.IncludeCockpit = includeCockpit
	if !cfg.Setup.IncludeCockpit {
		return nil
	}

	if err := p.printSection("Cockpit"); err != nil {
		return err
	}

	cockpitPort, err := p.askPort("Cockpit port", cfg.Ports.Cockpit)
	if err != nil {
		return err
	}
	cfg.Ports.Cockpit = cockpitPort

	installCockpit, err := p.askBool("Install Cockpit during setup", cfg.Setup.InstallCockpit)
	if err != nil {
		return err
	}
	cfg.Setup.InstallCockpit = installCockpit

	return nil
}

func (p promptSession) askString(label, defaultValue string, validate func(string) error) (string, error) {
	for {
		prompt := label
		if defaultValue != "" {
			prompt = fmt.Sprintf("%s [%s]", label, defaultValue)
		}
		if _, err := fmt.Fprintf(p.out, "%s: ", prompt); err != nil {
			return "", err
		}

		raw, err := p.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}

		value := strings.TrimSpace(raw)
		if value == "" {
			value = defaultValue
		}

		if validate != nil {
			if validateErr := validate(value); validateErr != nil {
				if _, err := fmt.Fprintf(p.out, "%s\n", validateErr.Error()); err != nil {
					return "", err
				}
				if err == io.EOF {
					return "", validateErr
				}
				continue
			}
		}

		return value, nil
	}
}

func (p promptSession) askInt(label string, defaultValue int, validate func(int) error) (int, error) {
	for {
		value, err := p.askString(label, strconv.Itoa(defaultValue), nonEmpty)
		if err != nil {
			return 0, err
		}

		parsed, err := strconv.Atoi(value)
		if err != nil {
			if _, writeErr := fmt.Fprintf(p.out, "Enter a valid number.\n"); writeErr != nil {
				return 0, writeErr
			}
			continue
		}

		if validate != nil {
			if validateErr := validate(parsed); validateErr != nil {
				if _, err := fmt.Fprintf(p.out, "%s\n", validateErr.Error()); err != nil {
					return 0, err
				}
				continue
			}
		}

		return parsed, nil
	}
}

func (p promptSession) askBool(label string, defaultValue bool) (bool, error) {
	defaultToken := "y/N"
	if defaultValue {
		defaultToken = "Y/n"
	}

	for {
		if _, err := fmt.Fprintf(p.out, "%s [%s]: ", label, defaultToken); err != nil {
			return false, err
		}

		raw, err := p.reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}

		value := strings.TrimSpace(strings.ToLower(raw))
		switch value {
		case "":
			return defaultValue, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			if _, err := fmt.Fprintln(p.out, "Enter y or n."); err != nil {
				return false, err
			}
			if err == io.EOF {
				return false, fmt.Errorf("invalid boolean answer: %s", value)
			}
		}
	}
}

func (p promptSession) askPort(label string, defaultValue int) (int, error) {
	return p.askInt(label, defaultValue, validPort)
}

func (p promptSession) askStackDir(defaultValue string) (string, error) {
	for {
		value, err := p.askString("Stack directory", defaultValue, nonEmpty)
		if err != nil {
			return "", err
		}

		absPath, err := filepath.Abs(value)
		if err != nil {
			return "", fmt.Errorf("resolve stack directory %q: %w", value, err)
		}

		info, err := os.Stat(absPath)
		if err == nil && info.IsDir() {
			return absPath, nil
		}

		ok, err := p.askBool(fmt.Sprintf("Directory %q does not exist yet. Use it anyway", absPath), false)
		if err != nil {
			return "", err
		}
		if ok {
			return absPath, nil
		}
	}
}

func nonEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("value must not be empty")
	}

	return nil
}

func validStackName(value string) error {
	if err := nonEmpty(value); err != nil {
		return err
	}

	return ValidateStackName(value)
}

func positiveInt(value int) error {
	if value <= 0 {
		return fmt.Errorf("value must be greater than zero")
	}

	return nil
}

func validPort(value int) error {
	if value < 1 || value > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	return nil
}
