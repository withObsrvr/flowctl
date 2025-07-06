# Flowctl Demo Guide

This guide provides ready-to-use demos for showcasing flowctl's capabilities. Perfect for sharing in developer communities, presentations, or quick testing.

## Quick Start Demos

### 1. Ultra-Simple One-Liner (30 seconds)

The fastest way to see flowctl in action:

```bash
# Download and run a pre-configured demo
curl -sL https://raw.githubusercontent.com/withobsrvr/flowctl/main/examples/discord-demo.sh | bash
```

This demo:
- Sets up a mock Stellar data source
- Processes ledger events every 2 seconds
- Displays formatted output with payment summaries
- Requires only flowctl to be installed

### 2. Copy-Paste Pipeline (1 minute)

Create and run a simple pipeline directly in your terminal:

```bash
# Create a demo pipeline
cat > demo.yaml << 'EOF'
version: 0.1
source:
  type: mock
  params:
    interval_ms: 1000
    data_template: '{"ledger": {{.Index}}, "ops": {{mul (mod .Index 10) 5}}, "asset": "XLM"}'
processors:
  - name: stellar_stats
    plugin: mock
    params:
      transform: '{"ledger": .ledger, "operations": .ops, "tps": {{div .ops 5.0}}}'
sink:
  type: stdout
  params:
    pretty: true
    prefix: "⭐ "
EOF

# Run it
flowctl run demo.yaml
```

This demonstrates:
- YAML-based pipeline configuration
- Data transformation with processors
- Real-time TPS (transactions per second) calculation
- Pretty-printed output with custom prefixes

### 3. Full Sandbox Experience (2-5 minutes)

Experience the complete flowctl ecosystem with supporting services:

```bash
# Clone and build flowctl
git clone https://github.com/withobsrvr/flowctl.git && cd flowctl
make build

# Start sandbox with example pipeline
./bin/flowctl sandbox start --pipeline examples/sandbox-pipeline.yaml

# Monitor the sandbox
./bin/flowctl sandbox status

# Stream logs in real-time
./bin/flowctl sandbox logs -f

# Stop when done
./bin/flowctl sandbox stop
```

The sandbox includes:
- Redis for event queuing
- ClickHouse for analytics storage
- Prometheus for metrics
- Grafana for visualization
- Kafka for event streaming
- PostgreSQL for metadata

### 4. Docker-Based Demo (No Installation Required)

Run flowctl without any local installation:

```bash
# Run a containerized demo
docker run -it ghcr.io/withobsrvr/flowctl:latest demo

# Or with a custom pipeline
docker run -it -v $(pwd):/workspace ghcr.io/withobsrvr/flowctl:latest run /workspace/my-pipeline.yaml
```

## Demo Scenarios

### Stellar Payment Tracker

Track and analyze Stellar payments in real-time:

```yaml
version: 0.1
name: payment-tracker

source:
  type: mock
  params:
    interval_ms: 3000
    data_template: |
      {
        "type": "payment",
        "from": "GA{{ printf "%04d" (mod .Index 1000) }}...",
        "to": "GB{{ printf "%04d" (add (mod .Index 1000) 1) }}...",
        "amount": {{ mul (add (mod .Index 100) 1) 10 }},
        "asset": {{ choice "XLM" "USDC" "BTC" "ETH" }},
        "memo": "Payment #{{ .Index }}"
      }

processors:
  - name: payment_enricher
    plugin: mock
    params:
      transform: |
        {
          "payment_id": {{ .Index }},
          "from_account": .from,
          "to_account": .to,
          "amount": .amount,
          "asset": .asset,
          "usd_value": {{ mul .amount 0.12 }},
          "risk_score": {{ mod .Index 10 }},
          "processed_at": "{{ now }}"
        }

sink:
  type: stdout
  params:
    pretty: true
    filter: "risk_score > 7"
```

### DEX Trade Monitor

Monitor decentralized exchange activity:

```yaml
version: 0.1
name: dex-monitor

source:
  type: mock
  params:
    interval_ms: 500
    data_template: |
      {
        "type": "trade",
        "base_asset": {{ choice "XLM" "USDC" }},
        "counter_asset": {{ choice "BTC" "ETH" "USDC" }},
        "base_amount": {{ mul (add (mod .Index 50) 1) 100 }},
        "counter_amount": {{ mul (add (mod .Index 50) 1) 12 }},
        "price": {{ div .counter_amount .base_amount }}
      }

processors:
  - name: trade_analytics
    plugin: mock
    params:
      aggregate_window: "1m"
      calculations:
        - volume_base
        - volume_counter
        - vwap
        - trade_count

sink:
  type: stdout
  params:
    pretty: true
    summary_only: true
```

## Sharing on Discord/Slack

### Compact Message Format

```
🌟 **Try Flowctl - Process Stellar Events in Real-Time!**

```bash
# 30-second demo
curl -sL https://bit.ly/flowctl-demo | bash
```

• Stream Stellar ledger data
• Transform with custom processors
• Route to any destination
• Zero dependencies with sandbox mode

GitHub: github.com/withobsrvr/flowctl
```

### Detailed Message Format

```
🚀 **Flowctl Demo - Stellar Event Processing**

**Quick Start** (copy & paste):
```yaml
# Save as demo.yaml
version: 0.1
source:
  type: mock
  params:
    interval_ms: 1000
processors:
  - name: stellar_processor
    plugin: mock
sink:
  type: stdout
```

```bash
flowctl run demo.yaml
```

**What you'll see:**
✅ Real-time Stellar event processing
✅ Configurable data pipelines
✅ Built-in monitoring & metrics

**Perfect for:**
• Payment tracking
• DEX analytics
• Ledger indexing
• Custom Stellar apps

**Full docs:** docs.flowctl.dev
```

## Tips for Effective Demos

1. **Start Simple**: Use the mock source for demos to avoid external dependencies
2. **Show Results Fast**: Keep interval_ms low (500-2000ms) for engaging demos
3. **Use Emojis**: Enable pretty printing with emojis for Discord/Slack appeal
4. **Highlight Use Cases**: Connect the demo to real Stellar applications
5. **Provide Next Steps**: Always include links to docs and more examples

## Troubleshooting Demo Issues

### "flowctl not found"
```bash
# Quick install
wget https://github.com/withobsrvr/flowctl/releases/latest/download/flowctl-linux-amd64
chmod +x flowctl-linux-amd64
sudo mv flowctl-linux-amd64 /usr/local/bin/flowctl
```

### "Permission denied"
```bash
# Use Docker instead
docker run -it ghcr.io/withobsrvr/flowctl:latest demo
```

### "Connection refused"
```bash
# Ensure sandbox services are running
flowctl sandbox status
flowctl sandbox start --services examples/sandbox.yaml
```

## Advanced Demo Features

### Live Configuration Reload
```bash
# Start with watch mode
flowctl run demo.yaml --watch

# Edit demo.yaml in another terminal
# Changes auto-reload without restart
```

### Multi-Pipeline Demo
```bash
# Run multiple pipelines simultaneously
flowctl sandbox start \
  --pipeline examples/stellar-pipeline.yaml \
  --pipeline examples/analytics-pipeline.yaml
```

### Performance Demo
```yaml
# High-throughput configuration
version: 0.1
source:
  type: mock
  params:
    interval_ms: 10
    batch_size: 100
processors:
  - name: batch_processor
    plugin: mock
    params:
      parallel: true
      workers: 4
sink:
  type: stdout
  params:
    stats_only: true
    stats_interval: "5s"
```

## Next Steps

After running demos, direct users to:

1. **Installation Guide**: `/docs/quickstart.md`
2. **Pipeline Examples**: `/examples/`
3. **API Documentation**: `/docs/api/`
4. **Community Discord**: `discord.gg/stellar-dev`

Remember: The best demo is one that solves a real problem. Customize these examples to match your audience's specific Stellar use cases!