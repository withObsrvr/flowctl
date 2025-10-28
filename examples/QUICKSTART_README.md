# Stellar Asset Balance Indexer Quickstart

Index Stellar account balances for any asset and query them with SQL.

**What you'll build:** A pipeline that processes Stellar ledgers, extracts account balances for specific assets (like USDC), and stores them in a queryable DuckDB database.

**Time to complete:** ~10 minutes

**Performance:** Process 100K ledgers in ~4 minutes, 1M ledgers in ~6 hours

---

## What You Get

After completing this quickstart, you'll have:

- ✅ A pipeline that indexes Stellar account balances
- ✅ SQL queryable database of account balances
- ✅ Ability to filter by specific assets (USDC, AQUA, etc.)
- ✅ Historical balance data for analysis

**Example queries you can run:**
```sql
-- Find all USDC holders
SELECT account_id, balance/10000000.0 as balance_usd
FROM account_balances
WHERE asset_code = 'USDC'
ORDER BY balance DESC;

-- Count trustlines by asset
SELECT asset_code, COUNT(*) as holders
FROM account_balances
GROUP BY asset_code
ORDER BY holders DESC;
```

---

## Prerequisites

### Required

- **Linux or macOS** (tested on Ubuntu 22.04, macOS 14+)
- **Go 1.21+** - [Install Go](https://go.dev/doc/install)
- **Git** - For cloning repositories
- **GCS credentials** - Access to Stellar data lake

### Recommended

- **16GB RAM** - For processing large ledger ranges
- **50GB disk space** - For database storage
- **Good internet connection** - Downloads ledger data from GCS

### Optional

- **DuckDB CLI** - For querying results (`brew install duckdb` or `apt install duckdb`)

---

## Quick Start

### 1. Clone & Build

```bash
# Clone repositories
git clone https://github.com/withobsrvr/flowctl.git
git clone https://github.com/withobsrvr/ttp-processor-demo.git

# Build flowctl
cd flowctl && make build && cd ..

# Build pipeline components
cd ttp-processor-demo/account-balance-processor && make build && cd ../..
cd ttp-processor-demo/duckdb-consumer && make build && cd ../..
cd ttp-processor-demo/stellar-live-source-datalake && make build && cd ../..
```

### 2. Configure GCS Access

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/your/service-account-key.json"
```

**Optional:** If you cloned the repositories to a non-standard location:
```bash
export TTP_PROCESSOR_DEMO_DIR="/custom/path/to/ttp-processor-demo"
```

**Note:** By default, `flowctl` looks for binaries in `../ttp-processor-demo` (sibling directory). The environment variable is only needed for custom setups.

### 3. Create Configuration

Create `quickstart.yaml`:

```yaml
apiVersion: flowctl.io/v1alpha1
kind: QuickstartPipeline
metadata:
  name: my-usdc-indexer

spec:
  source:
    type: stellar-live-source-datalake
    network: testnet
    ledgers:
      start: 3
      end: 10000  # 10K ledgers (~4 minutes)

  filters:
    assets:
      - code: USDC
        issuer: GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5

  processor:
    type: account-balance

  consumer:
    type: duckdb
    database:
      path: ./my_balances.duckdb
```

### 4. Run Pipeline

**Single command** (runs all 3 components):

```bash
cd flowctl
./bin/flowctl quickstart run --config ../quickstart.yaml
```

The pipeline will:
- ✅ Start stellar-live-source-datalake (GCS data source)
- ✅ Start account-balance-processor (balance extractor)
- ✅ Start duckdb-consumer (database writer)
- ✅ Monitor health endpoints
- ✅ Process ledgers 3-10000 (~4 minutes)

**Output:**
```
Starting Asset Balance Indexer quickstart: my-usdc-indexer
Network: testnet
Ledger range: 3-10000

Starting stellar-live-source-datalake...
  ✓ Source started (PID: 123456, health: http://localhost:8088/health)
  ✓ stellar-live-source-datalake is ready
Starting account-balance-processor...
  ✓ Processor started (PID: 123457, health: http://localhost:8089/health)
  ✓ account-balance-processor is ready
Starting duckdb-consumer...
  ✓ Consumer started (PID: 123458, health: http://localhost:8090/health)
  ✓ duckdb-consumer is ready

✅ All components running!

Pipeline status:
  Source:    http://localhost:8088/health
  Processor: http://localhost:8089/health
  Consumer:  http://localhost:8090/health

Press Ctrl+C to stop the pipeline
```

**When complete**, press Ctrl+C to stop all components gracefully.

### 5. Query Results

```bash
duckdb my_balances.duckdb
```

```sql
SELECT COUNT(*) FROM account_balances;  -- ~129 USDC trustlines

SELECT
    account_id,
    balance/10000000.0 as balance_usd
FROM account_balances
ORDER BY balance DESC
LIMIT 10;
```

---

## Example Configurations

See `examples/` directory:

- **usdc-only.yaml** - Index only USDC (simple)
- **all-assets.yaml** - Index everything
- **multi-asset.yaml** - USDC + AQUA + yXLM
- **mainnet-usdc.yaml** - Mainnet data
- **continuous.yaml** - Real-time streaming

---

## SQL Query Examples

### Basic Analysis

```sql
-- Total trustlines
SELECT COUNT(*) FROM account_balances;

-- Top assets by holders
SELECT
    asset_code,
    COUNT(DISTINCT account_id) as holders
FROM account_balances
GROUP BY asset_code
ORDER BY holders DESC;

-- Balance distribution
SELECT
    CASE
        WHEN balance = 0 THEN 'Zero'
        WHEN balance < 100000000 THEN '< 10'
        ELSE '> 10'
    END as range,
    COUNT(*) as accounts
FROM account_balances
WHERE asset_code = 'USDC'
GROUP BY range;
```

More examples in `queries.sql`.

---

## Performance

| Ledgers | Time | Database Size |
|---------|------|---------------|
| 10K | ~4 min | ~1MB |
| 100K | ~40 min | ~10MB |
| 1M | ~6.2 hours | ~100MB |

**Rate:** ~45 ledgers/second (I/O bound)

---

## Troubleshooting

### Config Errors

**"spec.processor.type must be [account-balance]"**
- Fix: Use `type: account-balance` (only supported processor)

**"spec.source.ledgers.start is required"**
- Fix: Add `ledgers: {start: 3, end: 10000}` to config

### Connection Errors

**"failed to connect to raw ledger source"**
- Check stellar-live-source-datalake is running: `curl localhost:50053`
- Check ports: `lsof -i :50053`

**"failed to access GCS bucket"**
- Verify: `echo $GOOGLE_APPLICATION_CREDENTIALS`
- Test: `gsutil ls gs://obsrvr-stellar-ledger-data-testnet-data/`

### Performance Issues

**Too slow?**
- Increase batch size to 10,000
- Filter specific assets instead of `indexAll`
- Run closer to GCS (us-central1)

**Out of memory?**
- Reduce batch size to 1,000
- Process fewer ledgers at once

---

## Database Schema

```sql
CREATE TABLE account_balances (
    account_id VARCHAR NOT NULL,
    asset_code VARCHAR NOT NULL,
    asset_issuer VARCHAR NOT NULL,
    balance BIGINT NOT NULL,              -- In stroops (÷ 10^7)
    last_modified_ledger INTEGER NOT NULL,
    inserted_at TIMESTAMP,
    PRIMARY KEY (account_id, asset_code, asset_issuer)
);
```

**Note:** Balance is in stroops. Divide by 10,000,000 for human-readable amounts.

---

## Architecture

```
stellar-live-source-datalake (:50053)
    ↓ gRPC: StreamRawLedgers
account-balance-processor (:50054)
    ↓ gRPC: StreamAccountBalances
duckdb-consumer
    ↓ Writes to DuckDB
SQL Queries
```

---

## Learn the Architecture: Manual Setup

Want to understand how the components work together? Run them manually in 3 terminals:

**Terminal 1 - Data Source:**
```bash
cd ttp-processor-demo/stellar-live-source-datalake
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/key.json"
export BACKEND_TYPE=ARCHIVE
export ARCHIVE_STORAGE_TYPE=GCS
export ARCHIVE_BUCKET_NAME="obsrvr-stellar-ledger-data-testnet-data"
export ARCHIVE_PATH="landing/ledgers/testnet"
export NETWORK_PASSPHRASE="Test SDF Network ; September 2015"
export PORT=50053
export HEALTH_PORT=8088
./go/stellar-live-source-datalake
```

**Terminal 2 - Processor:**
```bash
cd ttp-processor-demo/account-balance-processor
./bin/account-balance-processor -config ../../quickstart.yaml
```

**Terminal 3 - Consumer:**
```bash
cd ttp-processor-demo/duckdb-consumer
./bin/duckdb-consumer -config ../../quickstart.yaml
```

This gives you full visibility into each component's logs and behavior.

---

## FAQ

**Q: Can I use mainnet?**
A: Yes! Set `network: mainnet` in config.

**Q: Can I index multiple assets?**
A: Yes! Add multiple entries to `filters.assets`.

**Q: Can I use PostgreSQL?**
A: Coming soon! Schema already supports `consumer.type: postgresql`.

**Q: Continuous/real-time mode?**
A: Set `end_ledger: 0` for streaming.

---

## Next Steps

- Try `examples/all-assets.yaml` to index everything
- Export to CSV: `.mode csv` in DuckDB
- Run on mainnet for production data
- Check `queries.sql` for more SQL examples

---

**Version:** 1.0.0
**License:** [License]
**Support:** [GitHub Issues](https://github.com/withobsrvr/ttp-processor-demo/issues)
