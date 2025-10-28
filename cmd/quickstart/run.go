package quickstart

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// QuickstartConfig represents the minimal config structure we need to parse
type QuickstartConfig struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type Spec struct {
	Source    Source    `yaml:"source"`
	Processor Processor `yaml:"processor"`
	Consumer  Consumer  `yaml:"consumer"`
}

type Source struct {
	Network string        `yaml:"network"`
	Storage StorageConfig `yaml:"storage"`
	Ledgers LedgersConfig `yaml:"ledgers"`
}

type StorageConfig struct {
	Type   string `yaml:"type"`
	Bucket string `yaml:"bucket"`
	Path   string `yaml:"path"`
}

type LedgersConfig struct {
	Start uint32 `yaml:"start"`
	End   uint32 `yaml:"end"`
}

type Processor struct {
	GRPCPort   int `yaml:"grpcPort"`
	HealthPort int `yaml:"healthPort"`
}

type Consumer struct {
	HealthPort int `yaml:"healthPort"`
}

type runOptions struct {
	configFile string
}

func newRunCommand() *cobra.Command {
	opts := &runOptions{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the Asset Balance Indexer pipeline",
		Long: `Run the Asset Balance Indexer pipeline with all components.

This command:
1. Parses the quickstart.yaml configuration
2. Starts stellar-live-source-datalake (data source)
3. Starts account-balance-processor (processor)
4. Starts duckdb-consumer (consumer)
5. Monitors health endpoints
6. Handles graceful shutdown on Ctrl+C

All three components must be built before running this command.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuickstart(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.configFile, "config", "c", "quickstart.yaml", "Path to quickstart configuration file")

	return cmd
}

func runQuickstart(opts *runOptions) error {
	// 1. Load and parse config
	config, err := loadConfig(opts.configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Printf("Starting Asset Balance Indexer quickstart: %s\n", config.Metadata.Name)
	fmt.Printf("Network: %s\n", config.Spec.Source.Network)
	fmt.Printf("Ledger range: %d-%d\n", config.Spec.Source.Ledgers.Start, config.Spec.Source.Ledgers.End)
	fmt.Println()

	// Find component binaries
	binaries, err := findBinaries()
	if err != nil {
		return fmt.Errorf("failed to find binaries: %w", err)
	}

	// 2. Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// 3. Start components
	processes := make([]*exec.Cmd, 0, 3)

	// Start stellar-live-source-datalake
	fmt.Println("Starting stellar-live-source-datalake...")
	sourceCmd := buildSourceCommand(binaries.source, config)
	if err := sourceCmd.Start(); err != nil {
		return fmt.Errorf("failed to start source: %w", err)
	}
	processes = append(processes, sourceCmd)
	fmt.Printf("  ✓ Source started (PID: %d, health: http://localhost:8088/health)\n", sourceCmd.Process.Pid)

	// Wait for source to be ready
	if err := waitForHealthy("stellar-live-source-datalake", "http://localhost:8088/health", 30*time.Second); err != nil {
		killProcesses(processes)
		return err
	}

	// Start account-balance-processor
	fmt.Println("Starting account-balance-processor...")
	processorCmd := buildProcessorCommand(binaries.processor, opts.configFile)
	if err := processorCmd.Start(); err != nil {
		killProcesses(processes)
		return fmt.Errorf("failed to start processor: %w", err)
	}
	processes = append(processes, processorCmd)
	fmt.Printf("  ✓ Processor started (PID: %d, health: http://localhost:%d/health)\n",
		processorCmd.Process.Pid, config.Spec.Processor.HealthPort)

	// Wait for processor to be ready
	processorHealthURL := fmt.Sprintf("http://localhost:%d/health", config.Spec.Processor.HealthPort)
	if err := waitForHealthy("account-balance-processor", processorHealthURL, 30*time.Second); err != nil {
		killProcesses(processes)
		return err
	}

	// Start duckdb-consumer
	fmt.Println("Starting duckdb-consumer...")
	consumerCmd := buildConsumerCommand(binaries.consumer, opts.configFile)
	if err := consumerCmd.Start(); err != nil {
		killProcesses(processes)
		return fmt.Errorf("failed to start consumer: %w", err)
	}
	processes = append(processes, consumerCmd)
	fmt.Printf("  ✓ Consumer started (PID: %d, health: http://localhost:%d/health)\n",
		consumerCmd.Process.Pid, config.Spec.Consumer.HealthPort)

	// Wait for consumer to be ready
	consumerHealthURL := fmt.Sprintf("http://localhost:%d/health", config.Spec.Consumer.HealthPort)
	if err := waitForHealthy("duckdb-consumer", consumerHealthURL, 30*time.Second); err != nil {
		killProcesses(processes)
		return err
	}

	fmt.Println("\n✅ All components running!")
	fmt.Println("\nPipeline status:")
	fmt.Println("  Source:    http://localhost:8088/health")
	fmt.Printf("  Processor: http://localhost:%d/health\n", config.Spec.Processor.HealthPort)
	fmt.Printf("  Consumer:  http://localhost:%d/health\n", config.Spec.Consumer.HealthPort)
	fmt.Println("\nPress Ctrl+C to stop the pipeline")

	// 4. Wait for context cancellation or process exit
	done := make(chan error, 1)
	go func() {
		// Wait for any process to exit
		for _, proc := range processes {
			if err := proc.Wait(); err != nil {
				done <- fmt.Errorf("process exited: %w", err)
				return
			}
		}
		done <- nil
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Stopping all components...")
		killProcesses(processes)
		return nil
	case err := <-done:
		killProcesses(processes)
		if err != nil {
			return err
		}
		return nil
	}
}

func loadConfig(path string) (*QuickstartConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config QuickstartConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	if config.Spec.Processor.GRPCPort == 0 {
		config.Spec.Processor.GRPCPort = 50054
	}
	if config.Spec.Processor.HealthPort == 0 {
		config.Spec.Processor.HealthPort = 8089
	}
	if config.Spec.Consumer.HealthPort == 0 {
		config.Spec.Consumer.HealthPort = 8090
	}

	// Auto-configure storage based on network if not set
	if config.Spec.Source.Storage.Type == "" {
		config.Spec.Source.Storage.Type = "GCS"
	}
	if config.Spec.Source.Storage.Bucket == "" {
		if config.Spec.Source.Network == "mainnet" {
			config.Spec.Source.Storage.Bucket = "obsrvr-stellar-ledger-data-pubnet-data"
			config.Spec.Source.Storage.Path = "landing/ledgers/pubnet"
		} else {
			config.Spec.Source.Storage.Bucket = "obsrvr-stellar-ledger-data-testnet-data"
			config.Spec.Source.Storage.Path = "landing/ledgers/testnet"
		}
	}

	return &config, nil
}

type binaries struct {
	source    string
	processor string
	consumer  string
}

func findBinaries() (*binaries, error) {
	// Look for binaries in common locations
	baseDir := "/home/tillman/Documents/ttp-processor-demo"

	bins := &binaries{
		source:    filepath.Join(baseDir, "stellar-live-source-datalake/go/stellar-live-source-datalake"),
		processor: filepath.Join(baseDir, "account-balance-processor/bin/account-balance-processor"),
		consumer:  filepath.Join(baseDir, "duckdb-consumer/bin/duckdb-consumer"),
	}

	// Verify all binaries exist
	if _, err := os.Stat(bins.source); err != nil {
		return nil, fmt.Errorf("stellar-live-source-datalake binary not found at %s: %w", bins.source, err)
	}
	if _, err := os.Stat(bins.processor); err != nil {
		return nil, fmt.Errorf("account-balance-processor binary not found at %s: %w", bins.processor, err)
	}
	if _, err := os.Stat(bins.consumer); err != nil {
		return nil, fmt.Errorf("duckdb-consumer binary not found at %s: %w", bins.consumer, err)
	}

	return bins, nil
}

func buildSourceCommand(binaryPath string, config *QuickstartConfig) *exec.Cmd {
	cmd := exec.Command(binaryPath)

	// Build environment variables based on config
	envVars := []string{
		"BACKEND_TYPE=ARCHIVE",
		"ARCHIVE_STORAGE_TYPE=" + config.Spec.Source.Storage.Type,
		"ARCHIVE_BUCKET_NAME=" + config.Spec.Source.Storage.Bucket,
		"ARCHIVE_PATH=" + config.Spec.Source.Storage.Path,
		getNetworkPassphrase(config.Spec.Source.Network),
		"PORT=50053",
		"HEALTH_PORT=8088",
		"LEDGERS_PER_FILE=64",
		"FILES_PER_PARTITION=10",
	}

	// Add GOOGLE_APPLICATION_CREDENTIALS if it's set (needed for GCS)
	if gcpCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); gcpCreds != "" {
		envVars = append(envVars, "GOOGLE_APPLICATION_CREDENTIALS="+gcpCreds)
	}

	// Set environment (inherit parent environment + our additions)
	cmd.Env = append(os.Environ(), envVars...)

	// Forward stdout/stderr
	cmd.Stdout = prefixWriter("SOURCE", os.Stdout)
	cmd.Stderr = prefixWriter("SOURCE", os.Stderr)

	return cmd
}

func buildProcessorCommand(binaryPath, configPath string) *exec.Cmd {
	cmd := exec.Command(binaryPath, "-config", configPath)

	// Forward stdout/stderr
	cmd.Stdout = prefixWriter("PROCESSOR", os.Stdout)
	cmd.Stderr = prefixWriter("PROCESSOR", os.Stderr)

	return cmd
}

func buildConsumerCommand(binaryPath, configPath string) *exec.Cmd {
	cmd := exec.Command(binaryPath, "-config", configPath)

	// Forward stdout/stderr
	cmd.Stdout = prefixWriter("CONSUMER", os.Stdout)
	cmd.Stderr = prefixWriter("CONSUMER", os.Stderr)

	return cmd
}

func getNetworkPassphrase(network string) string {
	if network == "mainnet" {
		return "NETWORK_PASSPHRASE=Public Global Stellar Network ; September 2015"
	}
	return "NETWORK_PASSPHRASE=Test SDF Network ; September 2015"
}

func waitForHealthy(name, healthURL string, timeout time.Duration) error {
	fmt.Printf("  Waiting for %s to be ready...\n", name)

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("  ✓ %s is ready\n", name)
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("%s did not become healthy within %v", name, timeout)
}

func killProcesses(processes []*exec.Cmd) {
	for _, proc := range processes {
		if proc != nil && proc.Process != nil {
			_ = proc.Process.Kill()
		}
	}
}

// prefixWriter creates a writer that prefixes each line with a component name
func prefixWriter(prefix string, w io.Writer) io.Writer {
	r, pw := io.Pipe()

	go func() {
		defer pw.Close()
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				fmt.Fprintf(w, "[%s] %s", prefix, buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	return pw
}
