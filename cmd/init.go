package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	outputFile        string
	initNonInteractive bool
	networkFlag       string
	destinationFlag   string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new Stellar pipeline interactively",
	Long: `Interactive wizard to create your first Stellar data pipeline.

This command guides you through creating a v1 pipeline configuration with
automatic component downloads. Components are automatically downloaded from
ghcr.io when you run the pipeline.

Examples:
  # Interactive mode (default)
  flowctl init

  # Specify output file
  flowctl init --output my-pipeline.yaml

  # Non-interactive mode (use flags)
  flowctl init --non-interactive --network testnet --destination duckdb
`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file (default: <pipeline-name>.yaml)")
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "non-interactive mode (requires --network, --destination)")
	initCmd.Flags().StringVar(&networkFlag, "network", "", "network (testnet, mainnet)")
	initCmd.Flags().StringVar(&destinationFlag, "destination", "", "destination (postgres, duckdb, csv)")
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("\n🚀 Welcome to flowctl!")
	fmt.Println("Let's create your first Stellar data pipeline.")

	reader := bufio.NewReader(os.Stdin)

	// Collect inputs
	var blockchain, network, destination, name string
	var startLedger, endLedger string

	// flowctl is Stellar-only, so we hardcode this
	blockchain = "stellar"

	if initNonInteractive {
		// Validate required flags
		if networkFlag == "" || destinationFlag == "" {
			return fmt.Errorf("non-interactive mode requires --network and --destination flags")
		}
		network = networkFlag
		destination = destinationFlag
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
	options := []string{"PostgreSQL database", "DuckDB file", "CSV files"}

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
	destMap := []string{"postgres", "duckdb", "csv"}
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
			"description": fmt.Sprintf("Process %s data on %s", blockchain, network),
		},
		"spec": map[string]interface{}{
			"driver": "process",
		},
	}

	spec := pipeline["spec"].(map[string]interface{})

	// Add source component
	sourceConfig := map[string]interface{}{
		"network": network,
	}
	if startLedger != "" {
		sourceConfig["start_ledger"] = startLedger
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

	// Add sink component based on destination
	sinks := []map[string]interface{}{}

	switch toType {
	case "duckdb":
		sinks = append(sinks, map[string]interface{}{
			"id":   "duckdb-sink",
			"type": "duckdb-consumer@v1.0.0",
			"config": map[string]interface{}{
				"database_path": fmt.Sprintf("./%s.duckdb", name),
			},
			"inputs": []string{"stellar-source"},
		})
	case "postgres":
		sinks = append(sinks, map[string]interface{}{
			"id":   "postgres-sink",
			"type": "postgres-sink@v1.0.0",
			"config": map[string]interface{}{
				"connection_string": "postgresql://localhost:5432/stellar",
			},
			"inputs": []string{"stellar-source"},
		})
	case "csv":
		sinks = append(sinks, map[string]interface{}{
			"id":   "csv-sink",
			"type": "csv-sink@v1.0.0",
			"config": map[string]interface{}{
				"output_dir": "./data",
			},
			"inputs": []string{"stellar-source"},
		})
	}

	spec["sinks"] = sinks

	return pipeline
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

	fmt.Println("Your pipeline will:")
	fmt.Printf("  1. Read %s data (%s)\n", blockchain, network)
	fmt.Printf("  2. Send to %s\n", destination)

	// Show configuration details
	fmt.Println("\nPipeline configuration:")
	fmt.Printf("  Network: %s\n", network)

	// Check for ledger range in source config
	if sources, ok := spec["sources"].([]map[string]interface{}); ok && len(sources) > 0 {
		if config, ok := sources[0]["config"].(map[string]interface{}); ok {
			if startLedger, ok := config["start_ledger"].(string); ok && startLedger != "" {
				if endLedger, ok := config["end_ledger"].(string); ok && endLedger != "" {
					fmt.Printf("  Ledgers: %s to %s\n", startLedger, endLedger)
				} else {
					fmt.Printf("  Start Ledger: %s\n", startLedger)
				}
			} else {
				fmt.Println("  Mode: Continuous")
			}
		}
	}

	fmt.Println("  Format: v1 (with automatic component downloads)")

	fmt.Println("\nNext steps:")
	fmt.Printf("  $ flowctl run %s\n", filename)
	fmt.Println("\n  Components will be automatically downloaded from ghcr.io on first run!")

	fmt.Println("\nLearn more:")
	fmt.Println("  flowctl --help     # All commands")
	fmt.Println("  cat " + filename + "    # View your pipeline")

	fmt.Println("\nHappy data processing! 🎉")
}
