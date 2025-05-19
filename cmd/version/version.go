package version

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version is the version of the application
	Version = "dev"
	// Commit is the git commit hash of the application
	Commit = "none"
	// BuildDate is the date when the application was built
	BuildDate = "unknown"
)

// NewCommand creates the version command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Long:  `Print the version, git commit, and build date of the flowctl CLI.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("flowctl version %s\n", Version)
			fmt.Printf("  commit: %s\n", Commit)
			fmt.Printf("  built: %s\n", BuildDate)
			fmt.Printf("  go: %s\n", runtime.Version())
			fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}

	return cmd
}