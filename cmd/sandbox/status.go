package sandbox

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/sandbox/runtime"
	"github.com/withobsrvr/flowctl/internal/utils/logger"
	"go.uber.org/zap"
)

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show sandbox status",
		Long: `Display the status of all running sandbox containers, including:
- Container health and status
- Port mappings
- Service endpoints
- Resource usage`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}

	return cmd
}

func runStatus() error {
	logger.Info("Checking sandbox status")

	// Get runtime instance
	rt, err := runtime.GetExistingRuntime()
	if err != nil {
		return fmt.Errorf("failed to connect to runtime: %w", err)
	}

	// Get container status
	containers, err := rt.ListContainers()
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		fmt.Println("No sandbox containers running.")
		fmt.Println("Use 'flowctl sandbox start' to start the sandbox.")
		return nil
	}

	// Display status in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tPORTS\tIMAGE\tUPTIME")
	fmt.Fprintln(w, "----\t------\t-----\t-----\t------")

	for _, container := range containers {
		ports := formatPorts(container.Ports)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			container.Name,
			container.Status,
			ports,
			container.Image,
			container.Uptime,
		)
	}

	w.Flush()

	// Show network information
	network, err := rt.GetNetworkInfo()
	if err != nil {
		logger.Warn("Failed to get network info", zap.Error(err))
	} else {
		fmt.Printf("\nNetwork: %s\n", network.Name)
		fmt.Printf("Subnet: %s\n", network.Subnet)
	}

	return nil
}

func formatPorts(ports []runtime.PortMapping) string {
	if len(ports) == 0 {
		return "-"
	}

	result := ""
	for i, port := range ports {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%d:%d", port.Host, port.Container)
	}
	return result
}