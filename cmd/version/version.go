package version

import (
	"runtime"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/cliout"
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
		Long: `Print the version, git commit, and build date of the flowctl CLI.

Examples:
  # Human-readable
  flowctl version

  # Machine-readable, for a release check in CI (CLI-13)
  flowctl version --output json | jq -r '.data.version'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := cliout.FromContext(cmd.Context())
			return p.Object(cliout.Object{
				Schema: "flowctl.version.v1",
				Fields: []cliout.Field{
					{Label: "flowctl", Key: "version"},
					{Label: "commit", Key: "commit"},
					{Label: "built", Key: "build_date"},
					{Label: "go", Key: "go_version"},
					{Label: "os/arch", Key: "platform"},
				},
				Data: cliout.Row{
					"version":    Version,
					"commit":     Commit,
					"build_date": BuildDate,
					"go_version": runtime.Version(),
					"platform":   runtime.GOOS + "/" + runtime.GOARCH,
					"os":         runtime.GOOS,
					"arch":       runtime.GOARCH,
				},
			})
		},
	}

	return cmd
}
