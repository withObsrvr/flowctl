#!/bin/bash
# Flowctl Stellar Demo - Perfect for Discord!
# This demo shows how flowctl can process Stellar ledger data in real-time

echo "🚀 Welcome to Flowctl - Stellar Event Stream Processing!"
echo "This demo will:"
echo "  1. Set up a local sandbox environment"
echo "  2. Process mock Stellar ledger data"
echo "  3. Display processed events in real-time"
echo ""

# Create a simple pipeline that processes mock Stellar data
cat > stellar-demo.yaml << 'EOF'
version: 0.1
log_level: info
name: stellar-quick-demo

source:
  type: mock
  params:
    interval_ms: 2000
    data_template: |
      {
        "ledger": {{ .Index }},
        "hash": "{{ .Hash }}",
        "timestamp": "{{ .Timestamp }}",
        "operations": [
          {
            "type": "payment",
            "from": "GAXYZ{{ printf "%04d" (mod .Index 100) }}...",
            "to": "GBWXYZ{{ printf "%04d" (add (mod .Index 100) 1) }}...",
            "amount": {{ mul (add (mod .Index 10) 1) 100 }},
            "asset": "XLM"
          }
        ]
      }

processors:
  - name: stellar_analyzer
    plugin: mock
    params:
      transform: |
        {
          "ledger": .ledger,
          "total_operations": {{ len .operations }},
          "payment_summary": {
            "from": .operations[0].from,
            "to": .operations[0].to,
            "amount": .operations[0].amount,
            "asset": .operations[0].asset
          },
          "processed_at": "{{ now }}"
        }

sink:
  type: stdout
  params:
    pretty: true
    prefix: "🌟 STELLAR EVENT: "
EOF

# Check if flowctl is installed
if ! command -v flowctl &> /dev/null; then
    echo "❌ flowctl not found. Please install it first:"
    echo "   git clone https://github.com/withobsrvr/flowctl.git"
    echo "   cd flowctl && make build"
    echo "   export PATH=\$PATH:\$(pwd)/bin"
    exit 1
fi

echo "▶️  Starting Stellar event processing..."
echo "   Press Ctrl+C to stop"
echo ""

# Run the pipeline
flowctl run stellar-demo.yaml