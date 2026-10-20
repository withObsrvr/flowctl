package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/withobsrvr/flowctl/cmd/list"
	"github.com/withobsrvr/flowctl/cmd/processors"
	"github.com/withobsrvr/flowctl/cmd/sandbox"
	"github.com/withobsrvr/flowctl/cmd/server"
	"github.com/withobsrvr/flowctl/cmd/translate"
	"github.com/withobsrvr/flowctl/cmd/version"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

// envPrefix namespaces flowctl's environment variables. Without it viper's
// AutomaticEnv binds bare names like OUTPUT and CONTEXT, which collide with unrelated
// variables in any normal shell (CLI-9).
const envPrefix = "FLOWCTL"

var (
	cfgFile        string
	ctxName        string
	namespace      string
	output         string
	logLevel       string
	nonInteractive bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "flowctl",
	Short: "Control plane for data pipelines for the Stellar Blockchain",
	Long: `flowctl is a command-line tool for managing data pipelines for the Stellar Blockchain.
It provides a unified interface for developers, operators, and CI/CD jobs to interact with
a Flow-powered stack.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return setup(cmd)
	},
}

// Execute runs the root command and exits with the code its error maps to.
//
// Exit codes are a public contract (CLI-15); see internal/exitcode. Errors are
// reported on stderr so that stdout carries results only (CLI-5).
func Execute() {
	defer logger.Sync()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(exitcode.Of(err))
	}
}

// setup resolves configuration and installs the output printer before any command
// runs. It replaces cobra.OnInitialize, which cannot report an error and so had to
// call os.Exit directly, bypassing the exit-code contract.
func setup(cmd *cobra.Command) error {
	if err := initConfig(); err != nil {
		return err
	}

	format, err := cliout.ParseFormat(output)
	if err != nil {
		return err
	}
	cmd.SetContext(cliout.NewContext(cmd.Context(),
		cliout.New(format, cmd.OutOrStdout(), cmd.ErrOrStderr())))

	// Structured logs are diagnostics, not results, so they must not interleave with
	// machine-readable output on a pipe. They already go to stderr; in a machine
	// format we additionally drop below WARN unless the user asked for more, so that
	// `flowctl … --output json 2>/dev/null` is quiet without losing real problems.
	level := logLevel
	if format != cliout.FormatTable && !cmd.Flags().Changed("log-level") {
		level = "warn"
	}
	return logger.Init(level, logger.WithColor(cliout.ShouldColor(os.Stderr)))
}

func init() {
	// Flag parsing failures are usage errors, not generic failures (CLI-15).
	rootCmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		return exitcode.Wrap(exitcode.Usage, err)
	})

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/flowctl/flowctl.yaml)")
	rootCmd.PersistentFlags().StringVar(&ctxName, "context", "", "override current context")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "target Kubernetes namespace / logical tenant")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "output format (table|json|ndjson)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug|info|warn|error)")
	rootCmd.PersistentFlags().BoolVarP(&nonInteractive, "yes", "y", false, "assume yes for confirmations; never prompt")

	// Bind flags to viper
	_ = viper.BindPFlag("context", rootCmd.PersistentFlags().Lookup("context"))
	_ = viper.BindPFlag("namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	_ = viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))

	// Add commands
	rootCmd.AddCommand(list.NewCommand())
	rootCmd.AddCommand(processors.NewCommand())
	rootCmd.AddCommand(sandbox.NewCommand())
	rootCmd.AddCommand(translate.NewCommand())
	rootCmd.AddCommand(version.NewCommand())
	rootCmd.AddCommand(newConfigCommand())

	// Add hidden server command (for backward compatibility)
	serverCmd := server.NewCommand()
	serverCmd.Hidden = true
	rootCmd.AddCommand(serverCmd)
}

// configDir returns the directory flowctl reads its user config from, honouring
// XDG_CONFIG_HOME as CLI-9 requires and falling back to the spec's default.
func configDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "flowctl"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", exitcode.Wrap(exitcode.Usage,
			fmt.Errorf("cannot locate a config directory: %w; set XDG_CONFIG_HOME or pass --config", err))
	}
	return filepath.Join(home, ".config", "flowctl"), nil
}

// initConfig reads configuration in the precedence CLI-9 mandates: flags, then
// FLOWCTL_* environment variables, then the project file, then the user file, then
// defaults.
func initConfig() error {
	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		// An explicitly named config file that cannot be read is a usage error.
		// Only a discovered one may be absent.
		if err := viper.ReadInConfig(); err != nil {
			return exitcode.Wrap(exitcode.Usage, fmt.Errorf("reading config %s: %w", cfgFile, err))
		}
		return nil
	}

	// Lowest precedence first: the user file, then the project file merged over it.
	//
	// Config files are located by exact name rather than viper's SetConfigName
	// search. With a config type set, that search also matches an extension-less
	// file — and a developer working in the repo root has a `flowctl` *binary*
	// sitting there, which viper would happily try to parse as YAML.
	if dir, err := configDir(); err == nil {
		if err := mergeConfigFile(filepath.Join(dir, "flowctl.yaml"), false); err != nil {
			return err
		}
	}
	for _, name := range []string{"flowctl.yaml", "flowctl.yml"} {
		if err := mergeConfigFile(name, true); err != nil {
			return err
		}
	}
	if file := viper.ConfigFileUsed(); file != "" {
		logger.Info("Using config file", zap.String("file", file))
	}
	return nil
}

// mergeConfigFile layers path over whatever is already loaded. A file that is not
// there is normal and ignored; a file that is there and malformed is a usage error,
// because silently ignoring it would run the command with configuration the operator
// believes is in effect.
func mergeConfigFile(path string, merge bool) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	viper.SetConfigFile(path)
	read := viper.ReadInConfig
	if merge {
		read = viper.MergeInConfig
	}
	if err := read(); err != nil {
		return exitcode.Wrap(exitcode.Usage, fmt.Errorf("reading config %s: %w", path, err))
	}
	return nil
}
