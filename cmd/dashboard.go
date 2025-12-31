package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	flowctlpb "github.com/withobsrvr/flowctl/proto"
	flowctlv1 "github.com/withObsrvr/flow-proto/go/gen/flowctl/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	dashboardRefreshInterval time.Duration
)

// dashboardCmd shows an interactive terminal UI for monitoring pipelines
var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Interactive dashboard for monitoring pipelines",
	Long: `Launch an interactive terminal UI dashboard for monitoring pipeline runs
and component health in real-time.

Keyboard shortcuts:
  q         - Quit
  r         - Refresh now
  ↑/↓       - Navigate (future)

The dashboard auto-refreshes every second by default.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create the bubbletea program
		p := tea.NewProgram(initialModel(), tea.WithAltScreen())

		// Run the program
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running dashboard: %w", err)
		}

		return nil
	},
}

// Model for the dashboard
type dashboardModel struct {
	controlPlaneAddr string
	controlPlanePort int
	refreshInterval  time.Duration
	pipelines        []*flowctlpb.PipelineRun
	components       []*flowctlv1.ComponentStatusResponse
	lastUpdate       time.Time
	err              error
	quitting         bool
}

// Messages
type tickMsg time.Time
type dataMsg struct {
	pipelines  []*flowctlpb.PipelineRun
	components []*flowctlv1.ComponentStatusResponse
	err        error
}

func initialModel() dashboardModel {
	return dashboardModel{
		controlPlaneAddr: pipelinesControlPlaneAddr,
		controlPlanePort: pipelinesControlPlanePort,
		refreshInterval:  dashboardRefreshInterval,
		lastUpdate:       time.Now(),
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(m.refreshInterval),
		fetchDataCmd(m.controlPlaneAddr, m.controlPlanePort),
	)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			// Manual refresh
			return m, fetchDataCmd(m.controlPlaneAddr, m.controlPlanePort)
		}

	case tickMsg:
		// Auto-refresh on tick
		return m, tea.Batch(
			tickCmd(m.refreshInterval),
			fetchDataCmd(m.controlPlaneAddr, m.controlPlanePort),
		)

	case dataMsg:
		m.pipelines = msg.pipelines
		m.components = msg.components
		m.err = msg.err
		m.lastUpdate = time.Now()
		return m, nil
	}

	return m, nil
}

func (m dashboardModel) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	var s strings.Builder

	// Header
	s.WriteString(headerStyle.Render("FLOWCTL DASHBOARD"))
	s.WriteString("\n\n")

	// Show error if any
	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.err)))
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("Press 'q' to quit"))
		return s.String()
	}

	// Active Pipelines Section
	s.WriteString(sectionTitleStyle.Render("ACTIVE PIPELINES"))
	s.WriteString(fmt.Sprintf(" (%d running)", countRunning(m.pipelines)))
	s.WriteString("\n")
	s.WriteString(boxStyle.Render(renderActivePipelines(m.pipelines)))
	s.WriteString("\n\n")

	// Component Registry Section
	s.WriteString(sectionTitleStyle.Render("COMPONENT REGISTRY"))
	s.WriteString(fmt.Sprintf(" (%d registered)", len(m.components)))
	s.WriteString("\n")
	s.WriteString(boxStyle.Render(renderComponents(m.components)))
	s.WriteString("\n\n")

	// Footer with status and help
	lastUpdateStr := formatRunTime(m.lastUpdate)
	s.WriteString(helpStyle.Render(fmt.Sprintf("Last update: %s | [q]uit [r]efresh", lastUpdateStr)))

	return s.String()
}

// Commands

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchDataCmd(addr string, port int) tea.Cmd {
	return func() tea.Msg {
		endpoint := fmt.Sprintf("%s:%d", addr, port)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return dataMsg{err: fmt.Errorf("failed to connect to control plane at %s: %w", endpoint, err)}
		}
		defer conn.Close()

		// Fetch pipeline runs
		pbClient := flowctlpb.NewControlPlaneClient(conn)
		runsResp, err := pbClient.ListPipelineRuns(ctx, &flowctlpb.ListPipelineRunsRequest{
			Status: flowctlpb.RunStatus_RUN_STATUS_RUNNING,
			Limit:  100,
		})
		if err != nil {
			return dataMsg{err: fmt.Errorf("failed to list pipeline runs: %w", err)}
		}

		// Fetch components
		v1Client := flowctlv1.NewControlPlaneServiceClient(conn)
		compsResp, err := v1Client.ListComponents(ctx, &flowctlv1.ListComponentsRequest{
			IncludeUnhealthy: true,
		})
		if err != nil {
			return dataMsg{err: fmt.Errorf("failed to list components: %w", err)}
		}

		return dataMsg{
			pipelines:  runsResp.Runs,
			components: compsResp.Components,
		}
	}
}

// Rendering helpers

func renderActivePipelines(pipelines []*flowctlpb.PipelineRun) string {
	if len(pipelines) == 0 {
		return dimStyle.Render("No active pipeline runs")
	}

	var s strings.Builder

	for i, run := range pipelines {
		if i > 0 {
			s.WriteString("\n")
		}

		// Status indicator
		s.WriteString(activeStyle.Render("●"))
		s.WriteString(" ")
		s.WriteString(boldStyle.Render(run.PipelineName))
		s.WriteString("\n")

		// Details
		duration := formatRunDuration(run)
		events := formatEventCount(run.Metrics.EventsProcessed)
		throughput := fmt.Sprintf("%.1f/s", run.Metrics.EventsPerSecond)

		s.WriteString(dimStyle.Render(fmt.Sprintf("   Duration: %s  |  Events: %s  |  Rate: %s", duration, events, throughput)))

		if i < len(pipelines)-1 {
			s.WriteString("\n")
		}
	}

	return s.String()
}

func renderComponents(components []*flowctlv1.ComponentStatusResponse) string {
	if len(components) == 0 {
		return dimStyle.Render("No components registered")
	}

	var s strings.Builder

	for i, comp := range components {
		if i > 0 {
			s.WriteString("\n")
		}

		// Status indicator
		if comp.Status == flowctlv1.HealthStatus_HEALTH_STATUS_HEALTHY {
			s.WriteString(healthyStyle.Render("✓"))
		} else {
			s.WriteString(unhealthyStyle.Render("✗"))
		}
		s.WriteString(" ")

		// Component info
		s.WriteString(comp.Component.Id)
		s.WriteString(dimStyle.Render(fmt.Sprintf(" (%s)", comp.Component.Type.String())))

		// Last seen
		if comp.LastHeartbeat != nil {
			lastSeen := formatRunTime(comp.LastHeartbeat.AsTime())
			s.WriteString(dimStyle.Render(fmt.Sprintf("  •  last seen: %s", lastSeen)))
		}
	}

	return s.String()
}

func countRunning(pipelines []*flowctlpb.PipelineRun) int {
	count := 0
	for _, run := range pipelines {
		if run.Status == flowctlpb.RunStatus_RUN_STATUS_RUNNING {
			count++
		}
	}
	return count
}

// Styles

var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	successColor   = lipgloss.Color("#04B575")
	errorColor     = lipgloss.Color("#FF0000")
	dimColor       = lipgloss.Color("#666666")

	// Text styles
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		PaddingLeft(2)

	sectionTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor)

	boxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Width(80)

	boldStyle = lipgloss.NewStyle().
		Bold(true)

	dimStyle = lipgloss.NewStyle().
		Foreground(dimColor)

	activeStyle = lipgloss.NewStyle().
		Foreground(successColor).
		Bold(true)

	healthyStyle = lipgloss.NewStyle().
		Foreground(successColor).
		Bold(true)

	unhealthyStyle = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)

	errorStyle = lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true)

	helpStyle = lipgloss.NewStyle().
		Foreground(dimColor).
		Italic(true)
)

func init() {
	rootCmd.AddCommand(dashboardCmd)

	dashboardCmd.Flags().StringVar(&pipelinesControlPlaneAddr, "control-plane-address", "127.0.0.1", "Control plane address")
	dashboardCmd.Flags().IntVar(&pipelinesControlPlanePort, "control-plane-port", 8080, "Control plane port")
	dashboardCmd.Flags().DurationVar(&dashboardRefreshInterval, "refresh-interval", 1*time.Second, "Auto-refresh interval")
}
