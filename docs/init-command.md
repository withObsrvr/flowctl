# flowctl init Command Reference

The `flowctl init` command creates Stellar data pipeline configurations through an interactive wizard or via command-line flags.

## Synopsis

```bash
flowctl init [flags]
```

## Description

The init command guides you through creating a v1 pipeline configuration with automatic component downloads. Components are automatically downloaded from Docker Hub when you run the pipeline.

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--network` | string | (interactive) | Stellar network: `testnet` or `mainnet` |
| `--destination` | string | (interactive) | Sink type: `postgres`, `duckdb`, or `csv` |
| `--output`, `-o` | string | `stellar-pipeline.yaml` | Output filename |
| `--non-interactive` | bool | false | Skip interactive prompts (requires `--network` and `--destination`) |
| `-h`, `--help` | bool | false | Show help |

### Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `$HOME/.config/flowctl/flowctl.yaml` | Config file path |
| `--log-level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |

## Interactive Mode

Run without flags for an interactive wizard:

```bash
./bin/flowctl init
```

The wizard prompts for:

1. **Network Selection**
   - `testnet` - Stellar test network (recommended for development)
   - `mainnet` - Stellar production network

2. **Destination Selection**
   - `duckdb` - Embedded analytics database (easiest setup)
   - `postgres` - PostgreSQL database (production-ready)
   - `csv` - CSV file output (simplest format)

## Non-Interactive Mode

For automation, CI/CD, or scripting:

```bash
# Basic usage
./bin/flowctl init --non-interactive --network testnet --destination duckdb

# Custom output file
./bin/flowctl init --non-interactive --network mainnet --destination postgres -o prod-pipeline.yaml
```

## Examples

### Create a Testnet DuckDB Pipeline

```bash
./bin/flowctl init --non-interactive --network testnet --destination duckdb
```

Generates `stellar-pipeline.yaml`:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: stellar-pipeline
  description: Process stellar contract events on testnet

spec:
  driver: process

  sources:
    - id: stellar-source
      type: stellar-live-source@v1.0.0
      config:
        network_passphrase: "Test SDF Network ; September 2015"
        backend_type: RPC
        rpc_endpoint: https://soroban-testnet.stellar.org
        start_ledger: 54000000

  processors:
    - id: contract-events
      type: contract-events-processor@v1.0.0
      config:
        network_passphrase: "Test SDF Network ; September 2015"
      inputs: ["stellar-source"]

  sinks:
    - id: duckdb-sink
      type: duckdb-consumer@v1.0.0
      config:
        database_path: ./stellar-pipeline.duckdb
      inputs: ["contract-events"]
```

### Create a Mainnet PostgreSQL Pipeline

```bash
./bin/flowctl init --non-interactive --network mainnet --destination postgres -o mainnet.yaml
```

### Create a CSV Export Pipeline

```bash
./bin/flowctl init --non-interactive --network testnet --destination csv -o export.yaml
```

## Component Auto-Download

When you run a pipeline created by `flowctl init`, components are automatically downloaded from Docker Hub on first run.

### Available Components

| Component | Image | Type | Description |
|-----------|-------|------|-------------|
| Stellar Live Source | `docker.io/withobsrvr/stellar-live-source:v1.0.0` | Source | Streams Stellar ledger data in real-time |
| DuckDB Consumer | `docker.io/withobsrvr/duckdb-consumer:v1.0.0` | Sink | Writes data to embedded DuckDB database |
| PostgreSQL Sink | `docker.io/withobsrvr/postgres-sink:v1.0.0` | Sink | Writes data to PostgreSQL database |
| CSV Sink | `docker.io/withobsrvr/csv-sink:v1.0.0` | Sink | Writes data to CSV files |

### Registry Information

- **Registry**: Docker Hub (`docker.io`)
- **Organization**: `withobsrvr`
- **Cache Location**: `~/.flowctl/components/`

### Manual Download

To pre-download components:

```bash
docker pull docker.io/withobsrvr/stellar-live-source:v1.0.0
docker pull docker.io/withobsrvr/duckdb-consumer:v1.0.0
docker pull docker.io/withobsrvr/postgres-sink:v1.0.0
docker pull docker.io/withobsrvr/csv-sink:v1.0.0
```

## Generated Configuration Structure

The init command generates pipelines with a three-stage architecture: **source → processor → sink**.

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: <generated-name>
  description: <generated-description>

spec:
  driver: process

  sources:
    - id: stellar-source
      type: stellar-live-source@v1.0.0
      config:
        network_passphrase: <network-passphrase>
        backend_type: RPC
        rpc_endpoint: <rpc-endpoint>
        start_ledger: <recent-ledger>

  processors:
    - id: contract-events
      type: contract-events-processor@v1.0.0
      config:
        network_passphrase: <network-passphrase>
      inputs: ["stellar-source"]

  sinks:
    - id: <sink-id>
      type: <sink-type>
      config:
        <sink-specific-config>
      inputs: ["contract-events"]
```

### Configuration Notes

- **driver**: Uses `process` driver to run components as local processes
- **type**: Component type with version (e.g., `stellar-live-source@v1.0.0`)
- **processors**: The `contract-events-processor` extracts Soroban contract events from raw ledgers
- **inputs**: Connects each component to its upstream data source
- **Three-stage flow**: Source produces ledgers → Processor extracts events → Sink stores data

## Running the Pipeline

After creating a pipeline:

```bash
# Run the pipeline
./bin/flowctl run stellar-pipeline.yaml

# Run with debug logging
./bin/flowctl run stellar-pipeline.yaml --log-level=debug

# Dry run (validate only)
./bin/flowctl run --dry-run stellar-pipeline.yaml
```

## Next Steps

After creating your first pipeline:

1. **Run it**: `./bin/flowctl run stellar-pipeline.yaml`
2. **Monitor it**: `./bin/flowctl dashboard`
3. **Add processors**: Edit the YAML to add transformation stages
4. **Deploy to production**: `./bin/flowctl translate -f pipeline.yaml -o docker-compose`

## See Also

- [Quickstart Guide](../examples/quickstart/README.md)
- [Processor Discovery](processor-discovery.md)
- [Configuration Guide](configuration.md)
- [Building Components](building-components.md)
