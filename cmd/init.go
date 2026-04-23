package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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
  flowctl init --output my-pipeline.yaml

  # Non-interactive mode (use flags)
  flowctl init --non-interactive --network testnet --destination duckdb

  # Preset for the recommended first-run path
  flowctl init --preset testnet-duckdb
`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file (default: <pipeline-name>.yaml)")
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "non-interactive mode (requires --network/--destination unless --preset is used)")
	initCmd.Flags().StringVar(&networkFlag, "network", "", "network (testnet, mainnet)")
	initCmd.Flags().StringVar(&destinationFlag, "destination", "", "destination (postgres, duckdb)")
	initCmd.Flags().StringVar(&presetFlag, "preset", "", "starter preset (testnet-duckdb, testnet-postgres, mainnet-duckdb, mainnet-postgres)")
}

func runInit(cmd *cobra.Command, args []string) error {
	if !initNonInteractive && presetFlag == "" {
		fmt.Println("\n🚀 Welcome to flowctl!")
		fmt.Println("Let's create your first Stellar data pipeline.")
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

	fmt.Printf("\n✓ Created %s\n\n", filename)

	// Show summary
	printSummary(intent, filename, blockchain, network, destination)

	return nil
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

	fmt.Println("? Which network?")
	for i, opt := range options {
		prefix := "  "
		if i == 0 {
			prefix = ">"
		}
		fmt.Printf("  %s %d. %s\n", prefix, i+1, opt)
	}
	fmt.Print("Enter number [1]: ")

	choice := readIntWithDefault(reader, 1, 1, len(options))

	// Extract network name (strip description)
	networkMap := []string{"testnet", "mainnet"}
	selected := networkMap[choice-1]
	fmt.Printf("Selected: %s\n\n", options[choice-1])
	return selected
}

func promptDestination(reader *bufio.Reader) string {
	options := []string{"PostgreSQL database", "DuckDB file"}

	fmt.Println("? What do you want to do with the data?")
	for i, opt := range options {
		prefix := "  "
		if i == 0 {
			prefix = ">"
		}
		fmt.Printf("  %s %d. %s\n", prefix, i+1, opt)
	}
	fmt.Print("Enter number [1]: ")

	choice := readIntWithDefault(reader, 1, 1, len(options))

	// Map display names to event types
	destMap := []string{"postgres", "duckdb"}
	selected := destMap[choice-1]
	fmt.Printf("Selected: %s\n\n", options[choice-1])
	return selected
}

func promptLedgerRange(reader *bufio.Reader) [2]string {
	fmt.Println("? Ledger range (optional - press Enter to skip for continuous processing):")

	fmt.Print("  Start ledger [leave empty for continuous]: ")
	startLedger, _ := reader.ReadString('\n')
	startLedger = strings.TrimSpace(startLedger)

	var endLedger string
	if startLedger != "" {
		fmt.Print("  End ledger [leave empty for continuous]: ")
		endLedger, _ = reader.ReadString('\n')
		endLedger = strings.TrimSpace(endLedger)
	}

	if startLedger != "" && endLedger != "" {
		fmt.Printf("Selected: Ledgers %s to %s\n\n", startLedger, endLedger)
	} else {
		fmt.Println("Selected: Continuous processing")
	}

	return [2]string{startLedger, endLedger}
}

func promptName(reader *bufio.Reader, blockchain string) string {
	defaultName := blockchain + "-pipeline"
	fmt.Printf("? Pipeline name [%s]: ", defaultName)

	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	if name == "" {
		name = defaultName
	}

	fmt.Printf("Selected: %s\n\n", name)
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
		fmt.Printf("Invalid choice. Using default: %d\n", defaultVal)
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

	fmt.Println("Pipeline summary")
	fmt.Println("----------------")
	fmt.Printf("Source:      %s ledgers (%s)\n", blockchain, network)
	fmt.Println("Processor:   contract-events")
	fmt.Printf("Destination: %s\n", destination)
	fmt.Printf("File:        %s\n", filename)

	// Show configuration details
	if sources, ok := spec["sources"].([]map[string]interface{}); ok && len(sources) > 0 {
		if config, ok := sources[0]["config"].(map[string]interface{}); ok {
			if startLedger, ok := config["start_ledger"].(string); ok && startLedger != "" {
				if endLedger, ok := config["end_ledger"].(string); ok && endLedger != "" {
					fmt.Printf("Ledgers:     %s to %s\n", startLedger, endLedger)
				} else {
					fmt.Printf("Start at:    %s\n", startLedger)
				}
			} else {
				fmt.Println("Mode:        continuous")
			}
		}
	}
	fmt.Println("Format:      flowctl/v1")

	fmt.Println("\nNext:")
	fmt.Printf("  flowctl validate %s\n", filename)
	fmt.Printf("  flowctl run %s\n", filename)
	fmt.Println("\nThen, in another terminal:")
	fmt.Println("  flowctl status")
	fmt.Println("  flowctl pipelines active")

	if destination == "duckdb" {
		duckdbFile := strings.TrimSuffix(filename, ".yaml") + ".duckdb"
		fmt.Println("\nOptional, after stopping the pipeline:")
		fmt.Printf("  duckdb %s \"SELECT event_type, COUNT(*) FROM contract_events GROUP BY event_type\"\n", duckdbFile)
		fmt.Println("\nDuckDB is the recommended first-run path and does not require a local Docker daemon.")
	} else {
		fmt.Println("\nNote: ensure PostgreSQL is running and the sink credentials in the pipeline file are correct.")
	}

	fmt.Println("\nComponents are downloaded automatically on first run and cached under ~/.flowctl/.")
}
