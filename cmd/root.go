package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

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
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	defer logger.Sync()
	
	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/flowctl/flowctl.yaml)")
	rootCmd.PersistentFlags().StringVar(&ctxName, "context", "", "override current context")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "target Kubernetes namespace / logical tenant")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "output format (table|yaml|json|wide)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug|info|warn|error)")
	rootCmd.PersistentFlags().BoolVarP(&nonInteractive, "yes", "y", false, "non-interactive confirmations")

	// Bind flags to viper
	viper.BindPFlag("context", rootCmd.PersistentFlags().Lookup("context"))
	viper.BindPFlag("namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".flowctl" (without extension).
		viper.AddConfigPath(home + "/.config/flowctl")
		viper.SetConfigType("yaml")
		viper.SetConfigName("flowctl")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// Initialize the logger
	if err := logger.Init(logLevel); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		logger.Info("Using config file", zap.String("file", viper.ConfigFileUsed()))
	}
}
