#!/bin/bash
set -e

# Register the RawLedgerChunk schema
echo "Registering RawLedgerChunk schema..."
../bin/flowctl schemas push ../../stellar-live-source-datalake/protos/raw_ledger_service/raw_ledger_service.proto --package raw_ledger_service

# Register the TokenTransferEvent schema
echo "Registering TokenTransferEvent schema..."
../bin/flowctl schemas push ../../ttp-processor/protos/ingest/processors/token_transfer/token_transfer_event.proto --package token_transfer

echo "Schemas registered successfully!"