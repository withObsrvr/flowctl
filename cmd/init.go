package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/withobsrvr/flowctl/internal/apiversion"
	"github.com/withobsrvr/flowctl/internal/cliout"
	"github.com/withobsrvr/flowctl/internal/exitcode"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	outputFile         string
	initNonInteractive bool
	networkFlag        string
	destinationFlag    string
	presetFlag         string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new Stellar pipeline interactively",
	Long: `Interactive wizard to create your first Stellar data pipeline.

This command guides you through creating a v1 pipeline configuration with
automatic component downloads. Components are automatically downloaded from
Docker Hub when you run the pipeline.

Examples:
  # Interactive mode (default)
  flowctl init

  # Specify output file
  flowctl init --file my-pipeline.yaml

  # Non-interactive mode (use flags)
  flowctl init --non-interactive --network testnet --destination duckdb

  # Preset for the recommended first-run path
  flowctl init --preset testnet-duckdb
`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Renamed from --output, which CLI-3 reserves for selecting an output format.
	// No deprecated alias: a local --output would shadow the global one, so
	// `--output json` would silently be read as a filename — the exact confusion
	// the rule exists to prevent. Pre-1.0, VER-2 permits the clean break.
	initCmd.Flags().StringVarP(&outputFile, "file", "f", "", "file to write (default: <pipeline-name>.yaml)")
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "non-interactive mode (requires --network/--destination unless --preset is used)")
	initCmd.Flags().StringVar(&networkFlag, "network", "", "network (testnet, mainnet)")
	initCmd.Flags().StringVar(&destinationFlag, "destination", "", "destination (postgres, duckdb)")
	initCmd.Flags().StringVar(&presetFlag, "preset", "", "starter preset (testnet-duckdb, testnet-postgres, mainnet-duckdb, mainnet-postgres)")
}

// wizardOut is where the interactive wizard writes. Prompts, progress and the
// closing summary are all messages, not results, so they go to stderr and
// leave stdout for the record emitted at the end (CLI-5).
var wizardOut io.Writer = os.Stderr

func runInit(cmd *cobra.Command, args []string) error {
	p := cliout.FromContext(cmd.Context())
	wizardOut = p.Err()

	interactive := !initNonInteractive && presetFlag == ""

	// CLI-8: a tool must run without prompts when stdin is not a terminal.
	// Without this the wizard reads EOF for every answer and silently builds a
	// pipeline from defaults.
	if interactive && !cliout.IsTerminal(os.Stdin) {
		return exitcode.Errorf(exitcode.Usage,
			"flowctl init needs a terminal to prompt; use --preset, or --non-interactive with --network and --destination")
	}
	if interactive && p.Machine() {
		return exitcode.Errorf(exitcode.Usage,
			"--output %s cannot be combined with the interactive wizard; use --preset or --non-interactive", p.Format())
	}

	if interactive {
		fmt.Fprintln(wizardOut, "\n🚀 Welcome to flowctl!")
		fmt.Fprintln(wizardOut, "Let's create your first Stellar data pipeline.")
	}

	reader := bufio.NewReader(os.Stdin)

	// Collect inputs
	var blockchain, network, destination, name string
	var startLedger, endLedger string

	// flowctl is Stellar-only, so we hardcode this
	blockchain = "stellar"

	if initNonInteractive || presetFlag != "" {
		if presetFlag != "" {
			presetNetwork, presetDestination, err := resolveInitPreset(presetFlag)
			if err != nil {
				return err
			}
			network = presetNetwork
			destination = presetDestination
		} else {
			if networkFlag == "" || destinationFlag == "" {
				return fmt.Errorf("non-interactive mode requires --network and --destination flags unless --preset is used")
			}
			network = networkFlag
			destination = destinationFlag
		}
		if destination != "postgres" && destination != "duckdb" {
			return fmt.Errorf("unsupported destination %q (supported: postgres, duckdb)", destination)
		}
		name = fmt.Sprintf("%s-pipeline", blockchain)
	} else {
		// Interactive prompts
		network = promptNetwork(reader)
		destination = promptDestination(reader)
		ledgerRange := promptLedgerRange(reader)
		startLedger = ledgerRange[0]
		endLedger = ledgerRange[1]
		name = promptName(reader, blockchain)
	}

	// Map selections to event types
	fromType := getSourceEventType(blockchain)
	toType := getDestinationType(destination)

	// Generate intent topology
	intent := generateIntent(name, fromType, toType, network, startLedger, endLedger, blockchain)

	// Determine output filename
	filename := outputFile
	if filename == "" {
		filename = name + ".yaml"
	}

	// Write to file
	if err := writeIntentToFile(intent, filename); err != nil {
		return fmt.Errorf("failed to write pipeline file: %w", err)
	}

	fmt.Fprintf(wizardOut, "\n✓ Created %s\n\n", filename)

	// Show summary
	printSummary(intent, filename, blockchain, network, destination)

	row := cliout.Row{
		"file":        filename,
		"name":        name,
		"blockchain":  blockchain,
		"network":     network,
		"destination": destination,
		"api_version": apiversion.Current,
	}
	if startLedger != "" {
		row["start_ledger"] = startLedger
	}
	if endLedger != "" {
		row["end_ledger"] = endLedger
	}
	row["continuous"] = startLedger == ""

	// In table mode the summary above already told the operator everything, so
	// stdout carries just the path — which makes
	// `flowctl run "$(flowctl init --preset …)"` work.
	if !p.Machine() {
		_, err := fmt.Fprintln(p.Out(), filename)
		return err
	}
	return p.Object(cliout.Object{Schema: "flowctl.pipeline_init.v1", Data: row})
}

func resolveInitPreset(preset string) (network string, destination string, err error) {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "testnet-duckdb":
		return "testnet", "duckdb", nil
	case "testnet-postgres":
		return "testnet", "postgres", nil
	case "mainnet-duckdb":
		return "mainnet", "duckdb", nil
	case "mainnet-postgres":
		return "mainnet", "postgres", nil
	default:
		return "", "", fmt.Errorf("unsupported preset %q (supported: testnet-duckdb, testnet-postgres, mainnet-duckdb, mainnet-postgres)", preset)
	}
}

func promptNetwork(reader *bufio.Reader) string {
	options := []string{"Testnet (recommended for learning)", "Mainnet"}

	fmt.Fprintln(wizardOut, "? Which network?")
	for i, opt := range options {
		prefix := "  "
		if i == 0 {
			prefix = ">"
		}
		fmt.Fprintf(wizardOut, "  %s %d. %s\n", prefix, i+1, opt)
	}
	fmt.Fprint(wizardOut, "Enter number [1]: ")

	choice := readIntWithDefault(reader, 1, 1, len(options))

	// Extract network name (strip description)
	networkMap := []string{"testnet", "mainnet"}
	selected := networkMap[choice-1]
	fmt.Fprintf(wizardOut, "Selected: %s\n\n", options[choice-1])
	return selected
}

func promptDestination(reader *bufio.Reader) string {
	options := []string{"PostgreSQL database", "DuckDB file"}

	fmt.Fprintln(wizardOut, "? What do you want to do with the data?")
	for i, opt := range options {
		prefix := "  "
		if i == 0 {
			prefix = ">"
		}
		fmt.Fprintf(wizardOut, "  %s %d. %s\n", prefix, i+1, opt)
	}
	fmt.Fprint(wizardOut, "Enter number [1]: ")

	choice := readIntWithDefault(reader, 1, 1, len(options))

	// Map display names to event types
	destMap := []string{"postgres", "duckdb"}
	selected := destMap[choice-1]
	fmt.Fprintf(wizardOut, "Selected: %s\n\n", options[choice-1])
	return selected
}

func promptLedgerRange(reader *bufio.Reader) [2]string {
	fmt.Fprintln(wizardOut, "? Ledger range (optional - press Enter to skip for continuous processing):")

	fmt.Fprint(wizardOut, "  Start ledger [leave empty for continuous]: ")
	startLedger, _ := reader.ReadString('\n')
	startLedger = strings.TrimSpace(startLedger)

	var endLedger string
	if startLedger != "" {
		fmt.Fprint(wizardOut, "  End ledger [leave empty for continuous]: ")
		endLedger, _ = reader.ReadString('\n')
		endLedger = strings.TrimSpace(endLedger)
	}

	if startLedger != "" && endLedger != "" {
		fmt.Fprintf(wizardOut, "Selected: Ledgers %s to %s\n\n", startLedger, endLedger)
	} else {
		fmt.Fprintln(wizardOut, "Selected: Continuous processing")
	}

	return [2]string{startLedger, endLedger}
}

func promptName(reader *bufio.Reader, blockchain string) string {
	defaultName := blockchain + "-pipeline"
	fmt.Fprintf(wizardOut, "? Pipeline name [%s]: ", defaultName)

	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	if name == "" {
		name = defaultName
	}

	fmt.Fprintf(wizardOut, "Selected: %s\n\n", name)
	return name
}

func readIntWithDefault(reader *bufio.Reader, defaultVal, min, max int) int {
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < min || choice > max {
		fmt.Fprintf(wizardOut, "Invalid choice. Using default: %d\n", defaultVal)
		return defaultVal
	}

	return choice
}

func getSourceEventType(blockchain string) string {
	eventTypes := map[string]string{
		"stellar":  "stellar.ledger.v1",
		"ethereum": "ethereum.block.v1",
		"bitcoin":  "bitcoin.block.v1",
	}
	return eventTypes[blockchain]
}

func getDestinationType(destination string) string {
	// For now, we'll use simple destination names
	// In a full implementation, these would map to specific sink processors
	return destination
}

func generateIntent(name, fromType, toType, network, startLedger, endLedger, blockchain string) map[string]interface{} {
	// Generate v1 Pipeline with registry references
	pipeline := map[string]interface{}{
		"apiVersion": "flowctl/v1",
		"kind":       "Pipeline",
		"metadata": map[string]interface{}{
			"name":        name,
			"description": fmt.Sprintf("Process %s contract events on %s", blockchain, network),
		},
		"spec": map[string]interface{}{
			"driver": "process",
		},
	}

	spec := pipeline["spec"].(map[string]interface{})

	// Get network passphrase for processor config
	var networkPassphrase string
	if network == "testnet" {
		networkPassphrase = "Test SDF Network ; September 2015"
	} else {
		networkPassphrase = "Public Global Stellar Network ; September 2015"
	}

	// Add source component with RPC backend (easiest for quickstart)
	sourceConfig := map[string]interface{}{
		"network_passphrase": networkPassphrase,
		"backend_type":       "RPC",
	}
	// Set appropriate RPC endpoint based on network
	if network == "testnet" {
		sourceConfig["rpc_endpoint"] = "https://soroban-testnet.stellar.org"
	} else {
		sourceConfig["rpc_endpoint"] = "https://archive-rpc.lightsail.network"
	}
	// For RPC backend, start from a recent ledger (RPC nodes only keep ~24 hours of history)
	if startLedger != "" {
		sourceConfig["start_ledger"] = startLedger
	} else {
		// Query the RPC endpoint to get a recent ledger
		rpcEndpoint := sourceConfig["rpc_endpoint"].(string)
		if latestLedger := getLatestLedger(rpcEndpoint); latestLedger > 0 {
			// Start from 100 ledgers before the latest to give some buffer
			sourceConfig["start_ledger"] = latestLedger - 100
		}
	}
	if endLedger != "" {
		sourceConfig["end_ledger"] = endLedger
	}

	sources := []map[string]interface{}{
		{
			"id":     "stellar-source",
			"type":   "stellar-live-source@v1.0.0",
			"config": sourceConfig,
		},
	}
	spec["sources"] = sources

	// Add contract-events-processor to extract Soroban events from ledgers
	processors := []map[string]interface{}{
		{
			"id":   "contract-events",
			"type": "contract-events-processor@v1.0.0",
			"config": map[string]interface{}{
				"network_passphrase": networkPassphrase,
			},
			"inputs": []string{"stellar-source"},
		},
	}
	spec["processors"] = processors

	// Add sink component based on destination
	// Sinks receive from contract-events processor (not directly from source)
	sinks := []map[string]interface{}{}

	switch toType {
	case "duckdb":
		sinks = append(sinks, map[string]interface{}{
			"id":   "duckdb-sink",
			"type": "duckdb-consumer@v1.0.0",
			"config": map[string]interface{}{
				"database_path": fmt.Sprintf("./%s.duckdb", name),
			},
			"inputs": []string{"contract-events"},
		})
	case "postgres":
		sinks = append(sinks, map[string]interface{}{
			"id":   "postgres-sink",
			"type": "postgres-consumer@v1.0.0",
			"config": map[string]interface{}{
				"postgres_host":     "localhost",
				"postgres_port":     "5432",
				"postgres_db":       "stellar_events",
				"postgres_user":     "postgres",
				"postgres_password": "postgres",
			},
			"inputs": []string{"contract-events"},
		})
	}

	spec["sinks"] = sinks

	return pipeline
}

// getLatestLedger queries the RPC endpoint to get the latest ledger sequence
func getLatestLedger(rpcEndpoint string) int {
	client := &http.Client{Timeout: 10 * time.Second}

	// JSON-RPC request to get the latest ledger
	requestBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"getLatestLedger"}`)

	resp, err := client.Post(rpcEndpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to query RPC endpoint %s: %v\n", rpcEndpoint, err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Warning: RPC endpoint returned status %d\n", resp.StatusCode)
		return 0
	}

	var result struct {
		Result struct {
			Sequence int `json:"sequence"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to decode RPC response: %v\n", err)
		return 0
	}

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "Warning: RPC error %d: %s\n", result.Error.Code, result.Error.Message)
		return 0
	}

	return result.Result.Sequence
}

func writeIntentToFile(intent map[string]interface{}, filename string) error {
	data, err := yaml.Marshal(intent)
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func printSummary(intent map[string]interface{}, filename, blockchain, network, destination string) {
	spec := intent["spec"].(map[string]interface{})

	fmt.Fprintln(wizardOut, "Pipeline summary")
	fmt.Fprintln(wizardOut, "----------------")
	fmt.Fprintf(wizardOut, "Source:      %s ledgers (%s)\n", blockchain, network)
	fmt.Fprintln(wizardOut, "Processor:   contract-events")
	fmt.Fprintf(wizardOut, "Destination: %s\n", destination)
	fmt.Fprintf(wizardOut, "File:        %s\n", filename)

	// Show configuration details
	if sources, ok := spec["sources"].([]map[string]interface{}); ok && len(sources) > 0 {
		if config, ok := sources[0]["config"].(map[string]interface{}); ok {
			startLedger := configValueString(config["start_ledger"])
			endLedger := configValueString(config["end_ledger"])
			if startLedger != "" {
				if endLedger != "" {
					fmt.Fprintf(wizardOut, "Ledgers:     %s to %s\n", startLedger, endLedger)
				} else {
					fmt.Fprintf(wizardOut, "Start at:    %s\n", startLedger)
				}
			} else {
				fmt.Fprintln(wizardOut, "Mode:        continuous")
			}
		}
	}
	fmt.Fprintln(wizardOut, "Format:      flowctl/v1")

	fmt.Fprintln(wizardOut, "\nNext:")
	fmt.Fprintf(wizardOut, "  flowctl validate %s\n", filename)
	fmt.Fprintf(wizardOut, "  flowctl run %s\n", filename)
	fmt.Fprintln(wizardOut, "\nThen, in another terminal:")
	fmt.Fprintln(wizardOut, "  flowctl status")
	fmt.Fprintln(wizardOut, "  flowctl pipelines active")

	if destination == "duckdb" {
		duckdbFile := duckDBPathFromIntent(spec)
		fmt.Fprintln(wizardOut, "\nOptional, after stopping the pipeline:")
		fmt.Fprintf(wizardOut, "  duckdb %s \"SELECT event_type, COUNT(*) FROM contract_events GROUP BY event_type\"\n", duckdbFile)
		fmt.Fprintln(wizardOut, "\nDuckDB is the recommended first-run path and does not require a local Docker daemon.")
	} else {
		fmt.Fprintln(wizardOut, "\nNote: ensure PostgreSQL is running and the sink credentials in the pipeline file are correct.")
	}

	fmt.Fprintln(wizardOut, "\nComponents are downloaded automatically on first run and cached under ~/.flowctl/.")
}

func configValueString(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(value)
	default:
		return ""
	}
}

func duckDBPathFromIntent(spec map[string]interface{}) string {
	sinks, ok := spec["sinks"].([]map[string]interface{})
	if !ok {
		return "stellar-pipeline.duckdb"
	}
	for _, sink := range sinks {
		config, ok := sink["config"].(map[string]interface{})
		if !ok {
			continue
		}
		if path := configValueString(config["database_path"]); path != "" {
			return path
		}
	}
	return "stellar-pipeline.duckdb"
}
