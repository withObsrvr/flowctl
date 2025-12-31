package processors

import (
	"github.com/spf13/cobra"
)

var (
	endpoint       string
	includeUnhealthy bool
)

// NewCommand creates the processors command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "processors",
		Short: "Manage and discover processors",
		Long: `Manage processor components in the flowctl control plane.

Processors are components that transform events from one type to another.
This command group allows you to list, discover, and inspect processors
registered with the control plane.`,
	}

	// Add persistent flags for all subcommands
	cmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "e", "localhost:8080", "Control plane endpoint")
	cmd.PersistentFlags().BoolVar(&includeUnhealthy, "include-unhealthy", false, "Include unhealthy processors in results")

	// Add subcommands
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newShowCommand())
	cmd.AddCommand(newFindCommand())

	return cmd
}
