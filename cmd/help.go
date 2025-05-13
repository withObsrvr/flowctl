package cmd

import (
	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/cmd/help"
)

// helpCmd represents the help command
var helpCmd = &cobra.Command{
	Use:   "help [command]",
	Short: "Show help information",
	Long: `Show detailed help information for flowctl commands.
If no command is specified, shows general help information.`,
	Run: func(cmd *cobra.Command, args []string) {
		help.ShowCommandHelp(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(helpCmd)
}
