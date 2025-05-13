package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	showContext bool
	setContext  string
)

// contextCmd represents the context command
var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "View or switch current context",
	Long: `View or switch the current runtime context. A context defines the runtime environment
where pipelines will be deployed. This includes Kubernetes configuration, namespace,
and environment-specific settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showContext {
			currentContext := viper.GetString("context")
			if currentContext == "" {
				currentContext = "default"
			}
			fmt.Printf("Current context: %s\n", currentContext)
			return nil
		}

		if setContext != "" {
			// Update the context in the config
			viper.Set("context", setContext)
			if err := viper.WriteConfig(); err != nil {
				return fmt.Errorf("failed to update context: %w", err)
			}
			fmt.Printf("Switched to context: %s\n", setContext)
			return nil
		}

		// List available contexts
		contexts := viper.GetStringMap("contexts")
		if len(contexts) == 0 {
			fmt.Println("No contexts configured. Use 'flowctl context --set <name>' to create one.")
			return nil
		}

		fmt.Println("Available contexts:")
		for name, ctx := range contexts {
			ctxMap, ok := ctx.(map[string]interface{})
			if !ok {
				continue
			}
			namespace := ctxMap["namespace"]
			if namespace == nil {
				namespace = "default"
			}
			fmt.Printf("  %s (namespace: %s)\n", name, namespace)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.Flags().BoolVar(&showContext, "show", false, "show current context")
	contextCmd.Flags().StringVar(&setContext, "set", "", "set current context")
}
