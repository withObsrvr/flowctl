# Obsrvr Flow LCM‑to‑Parquet Ingestion & Horizon‑Style API Design

## 1 Purpose

Provide a repeatable, cloud‑efficient process that converts Stellar **LedgerCloseMeta** (LCM) files produced by **Galexie** into query‑ready **Parquet** partitions while delivering Horizon‑compatible REST endpoints—without running Horizon itself.

## 2 Scope

- **Networks:** Stellar Pubnet & Testnet (extensible).
- **Storage:** Google Cloud Storage (GCS) lake ➜ ClickHouse warm analytics ➜ optional Postgres hot cache.
- **Pipeline Runtime:** Obsrvr Flow (Nomad jobs).
- **API Façade:** FastAPI service exposing `/accounts`, `/transactions`, etc. with automatic tier routing.

## 3 Background & Drivers

1. Horizon will receive only protocol‑compat patches—no new features.
2. Full‑history Horizon ≥ 50 TB Postgres → high cloud cost.
3. Galexie already lands compressed XDR on GCS (Requester‑Pays).
4. Columnar Parquet + object storage slashes storage & scan cost while keeping sub‑minute freshness via micro‑batching.

## 4 High‑Level Architecture

```text
┌──────────────┐        Pub/Sub notif     ┌─────────────────┐
│  Galexie →  │ ───────────────────────▶ │  Obsrvr Flow    │
│  GCS Lake    │                         │  Pipeline       │
└──────────────┘                         │  (Nomad alloc)  │
                                         │  ┌───────────┐  │
                                         │  │DecodeLedger│  │
                                         │  ├───────────┤  │
                                         │  │ParquetWrit│  │
                                         │  └───────────┘  │
                                         └─────────┬──────┘
                                                   │ Parquet (≤60 s)   Iceberg commits
                                                   ▼
                                         gs://…/parquet/{network}/…
                                                   │  (external table)
                                            ClickHouse ↔ S3 engine
                                                   │
 API Clients ───► FastAPI Façade (auth+router) ─┬──► Postgres 30 d
                                               └──► ClickHouse 12 m
                                               └─ async Trino >12 m
```

## 5 Data Flow Details

1. **GCS Notification** fires when a new LCM object arrives.
2. **BufferedStorageSourceAdapter** streams bytes into Obsrvr Flow.
3. **DecodeLedger Processor**
   - Unmarshals V0/V1 unions.
   - Builds Arrow RecordBatches for **Ledgers, Transactions, Operations, Events, Upgrades**.
4. **ParquetWriter Processor**
   - Micro‑batch flush on `max_ledgers = 50` **OR** `max_seconds = 60`.
   - Writes partition `ledger_range=100k/ledger=58_013_050.parquet`.
   - Commits Iceberg metadata for schema evolution.
5. **Hourly Compaction Job** merges small files → 512 MB / 1 000‑ledger row‑groups.
6. **ClickHouse** external table reads Parquet on demand; Materialized Views create roll‑ups.
7. **API Façade** chooses backend based on `ledger_age`:
   - < 30 d → Postgres
   - 30 d–12 m → ClickHouse
   -
     >  12 m → start Trino job, return `202 Accepted` + poll URL.

## 6 Component Responsibilities

### 6.1 DecodeLedger Processor

| Concern        | Implementation                                                |
| -------------- | ------------------------------------------------------------- |
| XDR parse      | `xdr.SafeUnmarshal` → switch on `Type`                        |
| Arrow Schemas  | *Ledgers*, *Txns*, *Ops*, *Events*, *Upgrades* (see §7)       |
| Hashing        | compute `tx_hash` using network ID + envelope                 |
| Soroban Events | append to *Events* batch with `topic`, `value`, `contract_id` |
| Memory reuse   | builders retained & reset after each flush                    |

### 6.2 ParquetWriter Processor

- Configurable `max_ledgers`, `max_seconds`.
- Zstd compression level 3.
- Writes to GCS signed with Workload Identity.
- Publishes `ledger_seq` to Redis for health endpoints.

### 6.3 Compaction Job

- Iceberg `OPTIMIZE` every hour.
- Retain small files for ≤ 2 h to serve hot queries.

### 6.4 API Façade (FastAPI)

- Header `Authorization: Bearer <API_KEY>`.
- Mirrors Horizon paths.
- Uses async ClickHouse & Postgres drivers.
- Emits `x-obsrvr-reqid` & Prometheus metrics.

## 7 Arrow Schemas (v1)

```text
Ledgers
  ledger_seq      : int64   (PK)
  closed_at       : timestamp[ms, UTC]
  tx_count        : uint32
  protocol        : uint16
  bucket_bytes    : uint64?  -- nullable
Transactions
  ledger_seq      : int64
  tx_index        : uint32
  hash            : fixed_size_binary[32]
  source_account  : string
  fee_charged     : int64
  success         : bool
  memo_json       : string
Operations
  ledger_seq      : int64
  tx_hash         : fixed_size_binary[32]
  op_index        : uint16
  type            : uint16
  source_account  : string?
Events (Soroban)
  ledger_seq      : int64
  tx_hash         : fixed_size_binary[32]
  event_index     : uint16
  contract_id     : fixed_size_binary[32]
  topic0…topic3   : fixed_size_binary[32]?  (nullable)
  value_xdr       : binary
Upgrades
  ledger_seq      : int64
  upgrade_index   : uint8
  upgrade_type    : uint16
  params_xdr      : binary
```

## 8 Nomad Job Snippet (Obsrvr Flow)

```hcl
job "flow-ingest-pubnet" {
  group "pipeline" {
    task "decode" {
      driver = "docker"
      config {
        image = "obsrvr/flow:latest"
        command = "flowctl"
        args    = ["run", "/etc/flow/pipeline.yaml"]
      }
      template {
        data = <<EOF
processors:
  - type: DecodeLedger
  - type: ParquetWriter
    config:
      table_uri: "gcs://obsrvr-ledger/parquet/pubnet"
      max_ledgers: 50
      max_seconds: 60
EOF
        destination = "local/pipeline.yaml"
      }
      env { GCP_PROJECT = "obsrvr-prod" }
    }
  }
}
```

## 9 Deployment Phases

| Phase | Week | Milestones                                                   |
| ----- | ---- | ------------------------------------------------------------ |
|  0    |  1   | Bucket lifecycle rules; Pub/Sub → Nomad trigger              |
|  1    |  2   | DecodeLedger + ParquetWriter processors; backfill 90 d       |
|  2    |  3   | ClickHouse external table; roll‑ups; Grafana dashboard       |
|  3    |  4   | FastAPI façade for `/transactions/{hash}` & `/accounts/{id}` |
|  4    |  5   | Compaction job; async Trino cold queries; SLA docs           |

## 10 Cost Estimates (GCP, July‑2025)

- Hot Postgres 200 GB (NVMe): **\$34/mo**
- Warm Parquet 2 TB (Standard): **\$52/mo**
- Coldline 5 TB: **\$35/mo**
- ClickHouse serverless 2 CU: **\$59/mo**
- Cloud Run transforms: **\$10/mo**
- **Total ≈ \$190/mo** per network.

## 11 Testing & Validation

1. **Unit tests** for V0 & V1 decode paths (fixtures in `go‐xdr`).
2. Parquet ↔ Arrow round‑trip checksum.
3. ClickHouse query time < 75 ms for 30‑day window.
4. API façade returns identical JSON to public Horizon for same ledger.

## 12 Future Enhancements

- Add WebSocket tip feed via Redis Streams.
- Materialised liquidity‑pool stats.
- Per‑tenant filtered ingestion (asset allow‑list).
- Support for custom retention via Iceberg partition pruning.

## 13 References

- Galexie README
- apache/arrow v15 docs
- Iceberg spec v1.3
- Stellar Protocol 22 LCM XDR structs
- Obsrvr Flow `cdp-pipeline-workflow.md`

