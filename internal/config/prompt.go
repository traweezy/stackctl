package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func RunWizard(in io.Reader, out io.Writer, base Config) (Config, error) {
	session := promptSession{
		reader: bufio.NewReader(in),
		out:    out,
	}

	cfg := base

	stackName, err := session.askString("Stack name", cfg.Stack.Name, nonEmpty)
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

	postgresContainer, err := session.askString("Postgres container name", cfg.Services.PostgresContainer, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.PostgresContainer = postgresContainer

	postgresImage, err := session.askString("Postgres image", cfg.Services.Postgres.Image, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.Postgres.Image = postgresImage

	postgresDataVolume, err := session.askString("Postgres data volume", cfg.Services.Postgres.DataVolume, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.Postgres.DataVolume = postgresDataVolume

	maintenanceDatabase, err := session.askString("Postgres maintenance database", cfg.Services.Postgres.MaintenanceDatabase, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.Postgres.MaintenanceDatabase = maintenanceDatabase

	redisContainer, err := session.askString("Redis container name", cfg.Services.RedisContainer, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.RedisContainer = redisContainer

	redisImage, err := session.askString("Redis image", cfg.Services.Redis.Image, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.Redis.Image = redisImage

	redisDataVolume, err := session.askString("Redis data volume", cfg.Services.Redis.DataVolume, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.Redis.DataVolume = redisDataVolume

	redisAppendOnly, err := session.askBool("Enable Redis appendonly persistence", cfg.Services.Redis.AppendOnly)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.Redis.AppendOnly = redisAppendOnly

	redisSavePolicy, err := session.askString("Redis save policy", cfg.Services.Redis.SavePolicy, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.Redis.SavePolicy = redisSavePolicy

	redisMaxMemoryPolicy, err := session.askString("Redis maxmemory policy", cfg.Services.Redis.MaxMemoryPolicy, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Services.Redis.MaxMemoryPolicy = redisMaxMemoryPolicy

	includePgAdmin, err := session.askBool("Include pgAdmin in the stack", cfg.Setup.IncludePgAdmin)
	if err != nil {
		return Config{}, err
	}
	cfg.Setup.IncludePgAdmin = includePgAdmin

	if cfg.Setup.IncludePgAdmin {
		pgAdminContainer, err := session.askString("pgAdmin container name", cfg.Services.PgAdminContainer, nonEmpty)
		if err != nil {
			return Config{}, err
		}
		cfg.Services.PgAdminContainer = pgAdminContainer

		pgAdminImage, err := session.askString("pgAdmin image", cfg.Services.PgAdmin.Image, nonEmpty)
		if err != nil {
			return Config{}, err
		}
		cfg.Services.PgAdmin.Image = pgAdminImage

		pgAdminDataVolume, err := session.askString("pgAdmin data volume", cfg.Services.PgAdmin.DataVolume, nonEmpty)
		if err != nil {
			return Config{}, err
		}
		cfg.Services.PgAdmin.DataVolume = pgAdminDataVolume

		pgAdminServerMode, err := session.askBool("Run pgAdmin in server mode", cfg.Services.PgAdmin.ServerMode)
		if err != nil {
			return Config{}, err
		}
		cfg.Services.PgAdmin.ServerMode = pgAdminServerMode
	}

	postgresPort, err := session.askPort("Postgres port", cfg.Ports.Postgres)
	if err != nil {
		return Config{}, err
	}
	cfg.Ports.Postgres = postgresPort

	redisPort, err := session.askPort("Redis port", cfg.Ports.Redis)
	if err != nil {
		return Config{}, err
	}
	cfg.Ports.Redis = redisPort

	if cfg.Setup.IncludePgAdmin {
		pgAdminPort, err := session.askPort("pgAdmin port", cfg.Ports.PgAdmin)
		if err != nil {
			return Config{}, err
		}
		cfg.Ports.PgAdmin = pgAdminPort
	}

	cockpitPort, err := session.askPort("Cockpit port", cfg.Ports.Cockpit)
	if err != nil {
		return Config{}, err
	}
	cfg.Ports.Cockpit = cockpitPort

	postgresDatabase, err := session.askString("Postgres database name", cfg.Connection.PostgresDatabase, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Connection.PostgresDatabase = postgresDatabase

	postgresUsername, err := session.askString("Postgres username", cfg.Connection.PostgresUsername, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Connection.PostgresUsername = postgresUsername

	postgresPassword, err := session.askString("Postgres password", cfg.Connection.PostgresPassword, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.Connection.PostgresPassword = postgresPassword

	redisPassword, err := session.askString("Redis password (leave blank to disable auth)", cfg.Connection.RedisPassword, nil)
	if err != nil {
		return Config{}, err
	}
	cfg.Connection.RedisPassword = redisPassword

	if cfg.Setup.IncludePgAdmin {
		pgAdminEmail, err := session.askString("pgAdmin email", cfg.Connection.PgAdminEmail, nonEmpty)
		if err != nil {
			return Config{}, err
		}
		cfg.Connection.PgAdminEmail = pgAdminEmail

		pgAdminPassword, err := session.askString("pgAdmin password", cfg.Connection.PgAdminPassword, nonEmpty)
		if err != nil {
			return Config{}, err
		}
		cfg.Connection.PgAdminPassword = pgAdminPassword
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

	installCockpit, err := session.askBool("Install Cockpit during setup", cfg.Setup.InstallCockpit)
	if err != nil {
		return Config{}, err
	}
	cfg.Setup.InstallCockpit = installCockpit

	if !cfg.Stack.Managed {
		cfg.Setup.ScaffoldDefaultStack = false
	}

	packageManager, err := session.askString("Package manager", cfg.System.PackageManager, nonEmpty)
	if err != nil {
		return Config{}, err
	}
	cfg.System.PackageManager = packageManager

	cfg.ApplyDerivedFields()

	return cfg, nil
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
