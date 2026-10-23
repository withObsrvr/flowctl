package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/withobsrvr/flowctl/internal/cliout"
)

// settings are the configuration keys flowctl resolves through the precedence chain.
// Each is a global flag bound to viper in root.go.
var settings = []string{"context", "namespace", "output", "log-level"}

// newConfigCommand builds `flowctl config`.
func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect flowctl configuration",
		Long: `Inspect the configuration flowctl resolved for this invocation.

Configuration is resolved in this order, highest priority first:

  1. Command-line flags
  2. FLOWCTL_* environment variables
  3. A flowctl.yaml in the current directory
  4. A flowctl.yaml in $XDG_CONFIG_HOME/flowctl (or ~/.config/flowctl)
  5. Built-in defaults`,
	}
	cmd.AddCommand(newConfigShowCommand())
	return cmd
}

func newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show each setting and where its value came from",
		Long: `Print every configuration setting, its resolved value, and the source
that supplied it.

Examples:
  # See what flowctl thinks its configuration is
  flowctl config show

  # Check which file is being read
  flowctl config show --output json | jq -r '.items[] | select(.source=="config file")'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := cliout.FromContext(cmd.Context())

			rows := make([]cliout.Row, 0, len(settings))
			for _, key := range settings {
				value, source, origin := resolve(cmd, key)
				rows = append(rows, cliout.Row{
					"key":    key,
					"value":  value,
					"source": source,
					"origin": origin,
				})
			}

			if file := viper.ConfigFileUsed(); file != "" {
				p.Info("Config file: %s", file)
			} else {
				p.Info("Config file: none found")
			}

			return p.List(cliout.List{
				Schema:     "flowctl.config_setting_list.v1",
				ItemSchema: "flowctl.config_setting.v1",
				Columns: []cliout.Column{
					{Header: "Key", Key: "key"},
					{Header: "Value", Key: "value"},
					{Header: "Source", Key: "source"},
					{Header: "Origin", Key: "origin"},
				},
				Rows: rows,
			})
		},
	}
}

// resolve reports a setting's value, which layer supplied it, and the specific
// origin within that layer — the flag name, the environment variable, or the file.
func resolve(cmd *cobra.Command, key string) (value, source, origin string) {
	value = viper.GetString(key)

	if f := cmd.Flags().Lookup(key); f != nil && f.Changed {
		return value, "flag", "--" + key
	}
	if f := cmd.Root().PersistentFlags().Lookup(key); f != nil && f.Changed {
		return value, "flag", "--" + key
	}

	env := envPrefix + "_" + strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(key))
	if _, set := os.LookupEnv(env); set {
		return value, "environment", env
	}

	if viper.InConfig(key) {
		if file := viper.ConfigFileUsed(); file != "" {
			return value, "config file", file
		}
		return value, "config file", "unknown"
	}

	return value, "default", "built-in"
}
