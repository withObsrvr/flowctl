package sandbox

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
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
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cliout.FromContext(cmd.Context()))
		},
	}

	return cmd
}

func runStatus(p *cliout.Printer) error {
	logger.Info("Checking sandbox status")

	rt, err := runtime.GetExistingRuntime()
	if err != nil {
		return exitcode.Errorf(exitcode.Unavailable,
			"cannot reach the container runtime: %v", err)
	}

	containers, err := rt.ListContainers()
	if err != nil {
		return exitcode.Errorf(exitcode.Unavailable, "failed to list containers: %v", err)
	}

	rows := make([]cliout.Row, 0, len(containers))
	for _, container := range containers {
		ports := make([]map[string]any, 0, len(container.Ports))
		for _, port := range container.Ports {
			ports = append(ports, map[string]any{
				"host":      port.Host,
				"container": port.Container,
			})
		}
		rows = append(rows, cliout.Row{
			"name":   container.Name,
			"status": container.Status,
			"image":  container.Image,
			"uptime": container.Uptime,
			"ports":  ports,
			"_ports": formatPorts(container.Ports),
		})
	}

	if err := p.List(cliout.List{
		Schema:     "flowctl.sandbox_container_list.v1",
		ItemSchema: "flowctl.sandbox_container.v1",
		Columns: []cliout.Column{
			{Header: "Name", Key: "name"},
			{Header: "Status", Key: "status"},
			{Header: "Ports", Key: "_ports"},
			{Header: "Image", Key: "image"},
			{Header: "Uptime", Key: "uptime"},
		},
		Rows:  rows,
		Empty: "No sandbox containers running.\nUse 'flowctl sandbox start' to start the sandbox.",
	}); err != nil {
		return err
	}

	// Network details are context for a person, not part of the container
	// list, so they go to stderr and are omitted for a program (CLI-5).
	if !p.Machine() && len(rows) > 0 {
		network, err := rt.GetNetworkInfo()
		if err != nil {
			logger.Warn("Failed to get network info", zap.Error(err))
		} else {
			p.Info("\nNetwork: %s", network.Name)
			p.Info("Subnet: %s", network.Subnet)
		}
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
