# Arrow and Parquet Integration for Flowctl and TTP-Processor-Demo

## Executive Summary

This document outlines a comprehensive upgrade strategy for integrating Apache Arrow and Parquet into the existing flowctl and ttp-processor-demo systems. Based on analysis of the LCM-to-Parquet pipeline design and current architectures, we present a phased approach that maintains backward compatibility while adding high-performance columnar data processing capabilities.

## Overview and Integration Strategy

### Current State Analysis

**Flowctl Architecture:**
- Plugin-based pipeline orchestration with sources, processors, and sinks
- gRPC-based component communication using protobuf serialization
- DAG-based execution with typed event routing
- Support for Docker, Kubernetes, and Nomad deployment targets

**TTP-Processor-Demo Architecture:**
- Real-time streaming system for Token Transfer Protocol events
- XDR → gRPC → Consumer chain with row-based processing
- Multiple consumer applications (Go, Node.js, Rust/WASM)
- No current columnar data support

**Integration Goals:**
1. **Maintain Real-time Capabilities**: Preserve existing streaming architecture for low-latency use cases
2. **Add Columnar Processing**: Enable high-throughput batch processing for analytics workloads
3. **Support Hybrid Workloads**: Allow pipelines to serve both streaming and batch consumers
4. **Schema Evolution**: Handle protocol upgrades and data format changes gracefully
5. **Cost Optimization**: Reduce storage and compute costs through columnar compression

### Integration Strategy

The integration follows a **dual-mode architecture** that supports both streaming (row-oriented) and analytical (column-oriented) workloads:

```text
┌─────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│  Data Sources   │────▶│   Flowctl        │────▶│    Consumers     │
│                 │     │   Pipeline       │     │                  │
│ • Stellar RPC   │     │                  │     │ • Real-time Apps │
│ • Data Lake     │     │ ┌──────────────┐ │     │ • Analytics      │
│ • Live Streams  │     │ │ Arrow/Parquet│ │     │ • Data Warehouse │
└─────────────────┘     │ │  Processors  │ │     │ • ML Pipelines   │
                        │ └──────────────┘ │     └──────────────────┘
                        │                  │
                        │ ┌──────────────┐ │
                        │ │ Traditional  │ │
                        │ │ Processors   │ │
                        │ └──────────────┘ │
                        └──────────────────┘
```

## Arrow Schema Definitions for Stellar Data

### Core Stellar Data Types

Based on the parquet pipeline design and Stellar protocol structures, we define the following Arrow schemas:

#### 1. Ledger Schema

```go
var LedgerSchema = arrow.NewSchema(
    []arrow.Field{
        {Name: "ledger_seq", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "ledger_hash", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: false},
        {Name: "prev_hash", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: false},
        {Name: "closed_at", Type: arrow.FixedWidthTypes.Timestamp_ms, Nullable: false},
        {Name: "protocol_version", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "base_fee", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "base_reserve", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "max_tx_set_size", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "tx_count", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "op_count", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "bucket_list_hash", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: true},
        {Name: "bucket_list_size", Type: arrow.PrimitiveTypes.Uint64, Nullable: true},
    }, nil,
)
```

#### 2. Transaction Schema

```go
var TransactionSchema = arrow.NewSchema(
    []arrow.Field{
        {Name: "ledger_seq", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "tx_index", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "tx_hash", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: false},
        {Name: "source_account", Type: arrow.BinaryTypes.String, Nullable: false},
        {Name: "source_account_sequence", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
        {Name: "fee_charged", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
        {Name: "max_fee", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
        {Name: "operation_count", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "success", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
        {Name: "result_code", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
        {Name: "result_xdr", Type: arrow.BinaryTypes.Binary, Nullable: true},
        {Name: "envelope_xdr", Type: arrow.BinaryTypes.Binary, Nullable: false},
        {Name: "memo_type", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "memo_data", Type: arrow.BinaryTypes.Binary, Nullable: true},
        {Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ms, Nullable: false},
    }, nil,
)
```

#### 3. Operation Schema

```go
var OperationSchema = arrow.NewSchema(
    []arrow.Field{
        {Name: "ledger_seq", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "tx_hash", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: false},
        {Name: "tx_index", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "op_index", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "operation_type", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "source_account", Type: arrow.BinaryTypes.String, Nullable: true},
        {Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ms, Nullable: false},
        // Operation-specific fields as JSON for flexibility
        {Name: "operation_data", Type: arrow.BinaryTypes.String, Nullable: false},
        {Name: "operation_result", Type: arrow.BinaryTypes.String, Nullable: true},
    }, nil,
)
```

#### 4. Soroban Events Schema

```go
var SorobanEventSchema = arrow.NewSchema(
    []arrow.Field{
        {Name: "ledger_seq", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "tx_hash", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: false},
        {Name: "event_index", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "contract_id", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: false},
        {Name: "topic_0", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: true},
        {Name: "topic_1", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: true},
        {Name: "topic_2", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: true},
        {Name: "topic_3", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: true},
        {Name: "value_xdr", Type: arrow.BinaryTypes.Binary, Nullable: false},
        {Name: "event_type", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "in_successful_contract_call", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
        {Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ms, Nullable: false},
    }, nil,
)
```

#### 5. TTP Events Schema

```go
var TTPEventSchema = arrow.NewSchema(
    []arrow.Field{
        {Name: "ledger_seq", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "tx_hash", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: false},
        {Name: "event_index", Type: arrow.PrimitiveTypes.Uint32, Nullable: false},
        {Name: "contract_id", Type: &arrow.FixedSizeBinaryType{ByteWidth: 32}, Nullable: false},
        {Name: "event_type", Type: arrow.BinaryTypes.String, Nullable: false}, // "transfer", "mint", "burn", etc.
        {Name: "from_account", Type: arrow.BinaryTypes.String, Nullable: true},
        {Name: "to_account", Type: arrow.BinaryTypes.String, Nullable: true},
        {Name: "asset_code", Type: arrow.BinaryTypes.String, Nullable: false},
        {Name: "asset_issuer", Type: arrow.BinaryTypes.String, Nullable: true},
        {Name: "amount", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
        {Name: "memo", Type: arrow.BinaryTypes.String, Nullable: true},
        {Name: "created_at", Type: arrow.FixedWidthTypes.Timestamp_ms, Nullable: false},
    }, nil,
)
```

## Flowctl Component Upgrade Plan

### Phase 1: Core Arrow Infrastructure

#### 1.1 Arrow Schema Registry

Create a centralized schema management system:

```go
// internal/schema/registry.go
type SchemaRegistry interface {
    RegisterSchema(eventType string, schema *arrow.Schema) error
    GetSchema(eventType string, version uint32) (*arrow.Schema, error)
    EvolveSchema(eventType string, newSchema *arrow.Schema) (uint32, error)
    ListSchemas() ([]SchemaInfo, error)
}

type SchemaInfo struct {
    EventType string
    Version   uint32
    Schema    *arrow.Schema
    CreatedAt time.Time
}

type BoltSchemaRegistry struct {
    db *bolt.DB
}

func NewBoltSchemaRegistry(dbPath string) (*BoltSchemaRegistry, error) {
    // Implementation for persistent schema storage
}
```

#### 1.2 Arrow Event Types

Extend the existing event system to support Arrow records:

```go
// internal/events/arrow.go
type ArrowEvent struct {
    EventType string
    Record    arrow.Record
    Metadata  map[string]string
    Timestamp time.Time
}

func (ae *ArrowEvent) Serialize() ([]byte, error) {
    // Serialize Arrow record to IPC format
    var buf bytes.Buffer
    writer := ipc.NewWriter(&buf, ipc.WithSchema(ae.Record.Schema()))
    defer writer.Close()
    
    return buf.Bytes(), writer.Write(ae.Record)
}

func DeserializeArrowEvent(data []byte) (*ArrowEvent, error) {
    // Deserialize from IPC format
    buf := bytes.NewReader(data)
    reader, err := ipc.NewReader(buf)
    if err != nil {
        return nil, err
    }
    defer reader.Release()
    
    record, err := reader.Read()
    if err != nil {
        return nil, err
    }
    
    return &ArrowEvent{Record: record}, nil
}
```

#### 1.3 Arrow Processor Base

Create a base processor for Arrow transformations:

```go
// internal/processor/arrow.go
type ArrowProcessor struct {
    config       ArrowProcessorConfig
    inputSchema  *arrow.Schema
    outputSchema *arrow.Schema
    builder      *array.RecordBuilder
    batchSize    int64
    recordCount  int64
    pool         memory.Allocator
}

type ArrowProcessorConfig struct {
    InputEventType  string `yaml:"input_event_type"`
    OutputEventType string `yaml:"output_event_type"`
    BatchSize       int64  `yaml:"batch_size"`
    FlushInterval   string `yaml:"flush_interval"`
    Compression     string `yaml:"compression"`
}

func (ap *ArrowProcessor) Init(cfg map[string]any) error {
    // Initialize schema, allocator, and builder
    ap.pool = memory.NewGoAllocator()
    ap.builder = array.NewRecordBuilder(ap.pool, ap.outputSchema)
    return nil
}

func (ap *ArrowProcessor) Process(ctx context.Context, e *source.EventEnvelope) ([]*processor.Message, error) {
    // Transform individual events into Arrow records
    // Accumulate in builder until batch size reached
    if ap.recordCount >= ap.batchSize {
        return ap.flushBatch()
    }
    return nil, nil
}

func (ap *ArrowProcessor) flushBatch() ([]*processor.Message, error) {
    record := ap.builder.NewRecord()
    defer record.Release()
    
    arrowEvent := &ArrowEvent{
        EventType: ap.config.OutputEventType,
        Record:    record,
        Timestamp: time.Now(),
    }
    
    data, err := arrowEvent.Serialize()
    if err != nil {
        return nil, err
    }
    
    msg := &processor.Message{
        Type:    ap.config.OutputEventType,
        Payload: data,
    }
    
    ap.recordCount = 0
    ap.builder = array.NewRecordBuilder(ap.pool, ap.outputSchema)
    
    return []*processor.Message{msg}, nil
}
```

### Phase 2: Specialized Arrow Processors

#### 2.1 XDR-to-Arrow Processor

Transform Stellar XDR data into Arrow records:

```go
// processors/xdr_to_arrow.go
type XDRToArrowProcessor struct {
    ArrowProcessor
    xdrDecoder *xdr.Decoder
}

func (x *XDRToArrowProcessor) processLedger(lcm *xdr.LedgerCloseMeta) error {
    // Extract ledger data
    ledgerBuilder := x.builder.Field(0).(*array.Uint32Builder)
    hashBuilder := x.builder.Field(1).(*array.FixedSizeBinaryBuilder)
    timestampBuilder := x.builder.Field(2).(*array.TimestampBuilder)
    
    ledgerBuilder.Append(uint32(lcm.V0.LedgerHeader.LedgerSeq))
    hashBuilder.Append(lcm.V0.LedgerHeader.Hash[:])
    timestampBuilder.Append(arrow.Timestamp(lcm.V0.LedgerHeader.ScpValue.CloseTime) * 1000)
    
    // Process transactions
    for txIndex, tx := range lcm.V0.TxSet.Txs {
        if err := x.processTransaction(uint32(lcm.V0.LedgerHeader.LedgerSeq), txIndex, tx); err != nil {
            return err
        }
    }
    
    return nil
}

func (x *XDRToArrowProcessor) processTransaction(ledgerSeq uint32, txIndex int, envelope xdr.TransactionEnvelope) error {
    // Extract transaction data into Arrow builders
    // Handle different transaction types (V0, V1)
    // Process operations and extract TTP events
    
    return nil
}
```

#### 2.2 TTP Event Extractor

Specialized processor for Token Transfer Protocol events:

```go
// processors/ttp_extractor.go
type TTPExtractorProcessor struct {
    ArrowProcessor
    contractFilters map[string]bool
    eventTypes      map[string]bool
}

func (t *TTPExtractorProcessor) extractTTPEvents(txResult xdr.TransactionResult, contractEvents []xdr.ContractEvent) error {
    for eventIndex, event := range contractEvents {
        if t.isValidTTPEvent(event) {
            if err := t.addTTPEventToBuilder(eventIndex, event); err != nil {
                return err
            }
        }
    }
    return nil
}

func (t *TTPExtractorProcessor) isValidTTPEvent(event xdr.ContractEvent) bool {
    // Check if event matches TTP protocol patterns
    // Validate event structure and topics
    return true
}

func (t *TTPExtractorProcessor) addTTPEventToBuilder(index int, event xdr.ContractEvent) error {
    // Add TTP event data to Arrow record builder
    // Parse amounts, accounts, asset information
    // Handle different TTP event types (transfer, mint, burn, etc.)
    
    return nil
}
```

### Phase 3: Parquet Sink Implementation

#### 3.1 Parquet File Sink

```go
// internal/sink/parquet.go
type ParquetSink struct {
    config      ParquetSinkConfig
    writers     map[string]*pqarrow.FileWriter
    partitioner Partitioner
    metadata    map[string]string
}

type ParquetSinkConfig struct {
    BasePath        string                 `yaml:"base_path"`
    Compression     compress.Compression   `yaml:"compression"`
    PartitionBy     []string              `yaml:"partition_by"`
    WriterVersion   parquet.Version       `yaml:"writer_version"`
    RowGroupSize    int64                 `yaml:"row_group_size"`
    IcebergCatalog  *IcebergConfig        `yaml:"iceberg_catalog,omitempty"`
}

type IcebergConfig struct {
    CatalogType string `yaml:"catalog_type"` // "glue", "hive", "rest"
    CatalogURI  string `yaml:"catalog_uri"`
    Database    string `yaml:"database"`
    Table       string `yaml:"table"`
}

func (ps *ParquetSink) Write(ctx context.Context, msgs []*processor.Message) error {
    for _, msg := range msgs {
        arrowEvent, err := DeserializeArrowEvent(msg.Payload)
        if err != nil {
            return err
        }
        
        partition := ps.partitioner.GetPartition(arrowEvent)
        writer, err := ps.getOrCreateWriter(partition, arrowEvent.Record.Schema())
        if err != nil {
            return err
        }
        
        if err := writer.Write(arrowEvent.Record); err != nil {
            return err
        }
    }
    
    return nil
}

func (ps *ParquetSink) getOrCreateWriter(partition string, schema *arrow.Schema) (*pqarrow.FileWriter, error) {
    if writer, exists := ps.writers[partition]; exists {
        return writer, nil
    }
    
    // Create new Parquet file writer
    path := filepath.Join(ps.config.BasePath, partition+".parquet")
    file, err := os.Create(path)
    if err != nil {
        return nil, err
    }
    
    props := parquet.NewWriterProperties(
        parquet.WithCompression(ps.config.Compression),
        parquet.WithVersion(ps.config.WriterVersion),
    )
    
    writer, err := pqarrow.NewFileWriter(schema, file, props, pqarrow.DefaultWriterProps())
    if err != nil {
        return nil, err
    }
    
    ps.writers[partition] = writer
    return writer, nil
}
```

#### 3.2 Partitioning Strategy

```go
// internal/sink/partitioner.go
type Partitioner interface {
    GetPartition(event *ArrowEvent) string
}

type DateLedgerPartitioner struct {
    partitionFormat string
    ledgerRangeSize uint32
}

func (dlp *DateLedgerPartitioner) GetPartition(event *ArrowEvent) string {
    // Extract ledger sequence and timestamp
    record := event.Record
    
    // Get ledger_seq from first column (assuming it's always there)
    ledgerSeqArray := record.Column(0).(*array.Uint32)
    ledgerSeq := ledgerSeqArray.Value(0)
    
    // Get timestamp
    timestampArray := record.Column(1).(*array.Timestamp)
    timestamp := timestampArray.Value(0)
    
    // Create partition key: date=2024-01-20/ledger_range=58000000-58099999
    date := time.Unix(int64(timestamp)/1000, 0).Format("2006-01-02")
    ledgerRange := (ledgerSeq / dlp.ledgerRangeSize) * dlp.ledgerRangeSize
    
    return fmt.Sprintf("date=%s/ledger_range=%d-%d", 
        date, ledgerRange, ledgerRange+dlp.ledgerRangeSize-1)
}
```

### Phase 4: Arrow Flight RPC Integration

#### 4.1 Arrow Flight Service

```go
// internal/flight/service.go
type FlightService struct {
    flight.BaseFlightServer
    registry  SchemaRegistry
    dataStore DataStore
}

func (fs *FlightService) GetFlightInfo(ctx context.Context, request *flight.FlightDescriptor) (*flight.FlightInfo, error) {
    // Return metadata about available datasets
    // Include schema information and partitioning details
    
    schema, err := fs.registry.GetSchema(request.Type.String(), 1)
    if err != nil {
        return nil, err
    }
    
    endpoints := []*flight.FlightEndpoint{
        {
            Ticket: &flight.Ticket{
                Ticket: []byte(request.Path[0]),
            },
        },
    }
    
    return &flight.FlightInfo{
        Schema:    flight.SerializeSchema(schema, memory.DefaultAllocator),
        Endpoints: endpoints,
    }, nil
}

func (fs *FlightService) DoGet(tkt *flight.Ticket, stream flight.FlightService_DoGetServer) error {
    // Stream Arrow records back to client
    // Support predicate pushdown and column pruning
    
    records, err := fs.dataStore.GetRecords(string(tkt.Ticket))
    if err != nil {
        return err
    }
    
    for _, record := range records {
        if err := stream.Send(&flight.FlightData{
            DataHeader: flight.SerializeRecordBatch(record, memory.DefaultAllocator),
        }); err != nil {
            return err
        }
    }
    
    return nil
}
```

## TTP-Processor-Demo Upgrade Plan

### Phase 1: Add Arrow Output Mode

#### 1.1 Dual-Mode TTP Processor

Extend the existing TTP processor to support both streaming and batch modes:

```go
// ttp-processor/arrow_mode.go
type ArrowTTPProcessor struct {
    baseProcessor    *TTProcessor
    arrowEnabled     bool
    batchSize        int64
    flushInterval    time.Duration
    recordBuilder    *array.RecordBuilder
    recordCount      int64
    lastFlush        time.Time
    arrowClients     []TTPArrowClient
}

type TTPArrowClient interface {
    SendRecordBatch(batch arrow.Record) error
}

func (atp *ArrowTTPProcessor) ProcessLedger(ctx context.Context, ledger *RawLedger) error {
    // Process normally for streaming clients
    if err := atp.baseProcessor.ProcessLedger(ctx, ledger); err != nil {
        return err
    }
    
    // Add to Arrow batch if Arrow mode enabled
    if atp.arrowEnabled {
        if err := atp.addToArrowBatch(ledger); err != nil {
            return err
        }
        
        // Check if we should flush
        if atp.shouldFlush() {
            return atp.flushArrowBatch()
        }
    }
    
    return nil
}

func (atp *ArrowTTPProcessor) shouldFlush() bool {
    return atp.recordCount >= atp.batchSize || 
           time.Since(atp.lastFlush) >= atp.flushInterval
}

func (atp *ArrowTTPProcessor) flushArrowBatch() error {
    if atp.recordCount == 0 {
        return nil
    }
    
    record := atp.recordBuilder.NewRecord()
    defer record.Release()
    
    // Send to all Arrow clients
    for _, client := range atp.arrowClients {
        if err := client.SendRecordBatch(record); err != nil {
            return err
        }
    }
    
    // Reset builder
    atp.recordCount = 0
    atp.lastFlush = time.Now()
    atp.recordBuilder = array.NewRecordBuilder(memory.DefaultAllocator, TTPEventSchema)
    
    return nil
}
```

#### 1.2 Arrow gRPC Service

Add Arrow Flight RPC support to the TTP processor:

```go
// ttp-processor/flight_server.go
type TTPFlightServer struct {
    flight.BaseFlightServer
    processor *ArrowTTPProcessor
}

func (tfs *TTPFlightServer) DoGet(tkt *flight.Ticket, stream flight.FlightService_DoGetServer) error {
    // Handle real-time Arrow streaming
    // Support filtering by account, asset, event type
    
    filter, err := ParseTTPFilter(string(tkt.Ticket))
    if err != nil {
        return err
    }
    
    // Create filtered record stream
    recordChan := make(chan arrow.Record, 100)
    
    client := &FlightStreamClient{
        filter:     filter,
        recordChan: recordChan,
        stream:     stream,
    }
    
    tfs.processor.AddArrowClient(client)
    defer tfs.processor.RemoveArrowClient(client)
    
    // Stream records until client disconnects
    for record := range recordChan {
        data := &flight.FlightData{
            DataHeader: flight.SerializeRecordBatch(record, memory.DefaultAllocator),
        }
        
        if err := stream.Send(data); err != nil {
            return err
        }
    }
    
    return nil
}

type FlightStreamClient struct {
    filter     *TTPFilter
    recordChan chan arrow.Record
    stream     flight.FlightService_DoGetServer
}

func (fsc *FlightStreamClient) SendRecordBatch(batch arrow.Record) error {
    // Apply filter and send to channel
    filtered := fsc.applyFilter(batch)
    if filtered.NumRows() > 0 {
        select {
        case fsc.recordChan <- filtered:
            return nil
        default:
            return errors.New("client buffer full")
        }
    }
    return nil
}
```

### Phase 2: Consumer Application Updates

#### 2.1 Arrow-enabled Consumer (Go)

```go
// consumer_app/go_arrow/main.go
type ArrowTTPConsumer struct {
    flightClient flight.FlightServiceClient
    conn         *grpc.ClientConn
}

func (atc *ArrowTTPConsumer) ConsumeEvents(ctx context.Context, filter *TTPFilter) error {
    // Connect via Arrow Flight
    ticket := &flight.Ticket{
        Ticket: []byte(filter.Serialize()),
    }
    
    stream, err := atc.flightClient.DoGet(ctx, ticket)
    if err != nil {
        return err
    }
    
    for {
        data, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
        
        // Deserialize Arrow record
        record, err := flight.DeserializeRecordBatch(data.DataHeader, memory.DefaultAllocator)
        if err != nil {
            return err
        }
        
        // Process record batch
        if err := atc.processRecordBatch(record); err != nil {
            return err
        }
        
        record.Release()
    }
    
    return nil
}

func (atc *ArrowTTPConsumer) processRecordBatch(record arrow.Record) error {
    // Process batch of TTP events efficiently
    // Access columnar data directly for analytics
    
    ledgerSeqs := record.Column(0).(*array.Uint32)
    eventTypes := record.Column(4).(*array.String)
    amounts := record.Column(9).(*array.Int64)
    
    for i := 0; i < int(record.NumRows()); i++ {
        if !ledgerSeqs.IsNull(i) {
            ledgerSeq := ledgerSeqs.Value(i)
            eventType := eventTypes.Value(i)
            amount := amounts.Value(i)
            
            // Process individual event
            fmt.Printf("Ledger %d: %s event for %d stroops\n", 
                ledgerSeq, eventType, amount)
        }
    }
    
    return nil
}
```

#### 2.2 Arrow-enabled Consumer (Node.js)

```javascript
// consumer_app/node_arrow/index.js
const arrow = require('apache-arrow');
const grpc = require('@grpc/grpc-js');

class ArrowTTPConsumer {
    constructor(serverAddress) {
        this.client = new FlightServiceClient(
            serverAddress,
            grpc.credentials.createInsecure()
        );
    }
    
    async consumeEvents(filter) {
        const ticket = {
            ticket: Buffer.from(JSON.stringify(filter))
        };
        
        const stream = this.client.doGet(ticket);
        
        stream.on('data', (flightData) => {
            // Deserialize Arrow record
            const table = arrow.Table.from(flightData.dataHeader);
            
            // Process columnar data
            const ledgerSeqs = table.getColumn('ledger_seq');
            const eventTypes = table.getColumn('event_type');
            const amounts = table.getColumn('amount');
            
            for (let i = 0; i < table.numRows; i++) {
                const ledgerSeq = ledgerSeqs.get(i);
                const eventType = eventTypes.get(i);
                const amount = amounts.get(i);
                
                console.log(`Ledger ${ledgerSeq}: ${eventType} event for ${amount} stroops`);
            }
        });
        
        stream.on('end', () => {
            console.log('Stream ended');
        });
        
        stream.on('error', (err) => {
            console.error('Stream error:', err);
        });
    }
}
```

### Phase 3: Parquet Storage Integration

#### 3.1 TTP Event Archival

Add Parquet storage for historical TTP events:

```go
// ttp-processor/parquet_archiver.go
type ParquetArchiver struct {
    config      ParquetArchiverConfig
    writer      *pqarrow.FileWriter
    currentFile string
    recordCount int64
}

type ParquetArchiverConfig struct {
    OutputPath      string `yaml:"output_path"`
    PartitionBy     string `yaml:"partition_by"` // "daily", "hourly", "ledger_range"
    Compression     string `yaml:"compression"`
    RowGroupSize    int64  `yaml:"row_group_size"`
    MaxFileSize     int64  `yaml:"max_file_size"`
    IcebergEnabled  bool   `yaml:"iceberg_enabled"`
}

func (pa *ParquetArchiver) ArchiveRecord(record arrow.Record) error {
    // Check if we need to rotate file
    if pa.shouldRotateFile(record) {
        if err := pa.rotateFile(record); err != nil {
            return err
        }
    }
    
    // Write record to current file
    if err := pa.writer.Write(record); err != nil {
        return err
    }
    
    pa.recordCount += record.NumRows()
    return nil
}

func (pa *ParquetArchiver) shouldRotateFile(record arrow.Record) bool {
    if pa.writer == nil {
        return true
    }
    
    // Check file size, record count, or time-based rotation
    return pa.recordCount > pa.config.RowGroupSize
}

func (pa *ParquetArchiver) rotateFile(record arrow.Record) error {
    // Close current writer
    if pa.writer != nil {
        if err := pa.writer.Close(); err != nil {
            return err
        }
    }
    
    // Create new file with partition-based naming
    partition := pa.getPartition(record)
    filename := fmt.Sprintf("ttp_events_%s_%d.parquet", partition, time.Now().Unix())
    path := filepath.Join(pa.config.OutputPath, filename)
    
    // Create new writer
    file, err := os.Create(path)
    if err != nil {
        return err
    }
    
    props := parquet.NewWriterProperties(
        parquet.WithCompression(getCompression(pa.config.Compression)),
    )
    
    pa.writer, err = pqarrow.NewFileWriter(record.Schema(), file, props, pqarrow.DefaultWriterProps())
    if err != nil {
        return err
    }
    
    pa.currentFile = path
    pa.recordCount = 0
    
    return nil
}
```

## Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)
1. **Schema Registry Implementation**
   - Create Arrow schema definitions for Stellar data types
   - Implement persistent schema storage with BoltDB
   - Add schema evolution support

2. **Arrow Event System**
   - Extend flowctl event types to support Arrow records
   - Implement Arrow IPC serialization/deserialization
   - Add Arrow record validation

3. **Basic Arrow Processor**
   - Create ArrowProcessor base class
   - Implement record batching and flushing
   - Add configuration support for batch sizes and intervals

### Phase 2: Core Processing (Weeks 3-4)
1. **XDR-to-Arrow Processor**
   - Implement Stellar XDR parsing to Arrow records
   - Support all major Stellar data types (ledgers, transactions, operations)
   - Add efficient memory management and reuse

2. **TTP Arrow Processor**
   - Create specialized TTP event extractor
   - Support contract filtering and event type filtering
   - Implement dual-mode operation (streaming + batch)

3. **Parquet Sink**
   - Implement basic Parquet file writer
   - Add partitioning support (date, ledger range)
   - Support multiple compression algorithms

### Phase 3: Advanced Features (Weeks 5-6)
1. **Arrow Flight RPC**
   - Implement Flight server in flowctl
   - Add predicate pushdown and column pruning
   - Support real-time streaming over Flight

2. **TTP-Processor-Demo Integration**
   - Add Arrow output mode to existing TTP processor
   - Maintain backward compatibility with streaming clients
   - Implement Arrow Flight endpoint

3. **Consumer Updates**
   - Create Arrow-enabled consumer examples (Go, Node.js)
   - Demonstrate columnar data processing
   - Add performance benchmarks

### Phase 4: Production Features (Weeks 7-8)
1. **Iceberg Integration**
   - Add Iceberg table format support
   - Implement schema evolution
   - Support ACID transactions

2. **Cloud Storage**
   - Add native GCS/S3 Parquet sinks
   - Implement cloud-native partitioning
   - Support managed catalog services (Glue, Hive)

3. **Performance Optimization**
   - Add compression benchmarks
   - Implement memory pooling
   - Optimize for high-throughput scenarios

4. **Monitoring and Observability**
   - Add Arrow-specific metrics
   - Implement performance monitoring
   - Create operational dashboards

### Phase 5: Analytics Integration (Weeks 9-10)
1. **Query Engine Integration**
   - Add ClickHouse external table support
   - Implement DuckDB integration for local analytics
   - Support Trino/Presto for large-scale queries

2. **API Layer**
   - Create Horizon-compatible API over Parquet data
   - Implement automatic tier routing (hot/warm/cold)
   - Add GraphQL interface for flexible queries

3. **ML Pipeline Support**
   - Create feature extraction pipelines
   - Support for TensorFlow/PyTorch data loaders
   - Implement time-series analytics

## Configuration Examples

### Flowctl Pipeline with Arrow/Parquet

```yaml
apiVersion: flow.obsrvr.dev/v1
kind: Pipeline
metadata:
  name: stellar-arrow-pipeline
  namespace: default
spec:
  components:
    sources:
      - id: stellar-data-lake
        type: stellar-datastore
        config:
          backend: gcs
          bucket: stellar-ledgers
          prefix: ledgers/
          
    processors:
      - id: xdr-to-arrow
        type: xdr-arrow-processor
        inputEventTypes:
          - stellar.raw_ledger.v1
        outputEventTypes:
          - arrow.ledger.v1
          - arrow.transaction.v1
          - arrow.operation.v1
        config:
          batch_size: 1000
          flush_interval: "30s"
          
      - id: ttp-extractor
        type: ttp-arrow-processor
        inputEventTypes:
          - arrow.operation.v1
        outputEventTypes:
          - arrow.ttp_event.v1
        config:
          contract_filters:
            - "CBIELTK6YBZJU5UP2WWQEUCYKLPU6AUNZ2BQ4WWFEIE3USCIHMXQDAMA" # USDC
            - "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGQP6"  # XLM
          event_types:
            - "transfer"
            - "mint" 
            - "burn"
            
    sinks:
      - id: parquet-storage
        type: parquet-file
        inputEventTypes:
          - arrow.ledger.v1
          - arrow.transaction.v1
          - arrow.ttp_event.v1
        config:
          base_path: "gs://obsrvr-parquet/stellar"
          partition_by: ["network", "date", "ledger_range"]
          compression: "snappy"
          row_group_size: 100000
          iceberg:
            catalog_type: "glue"
            database: "stellar"
            table: "events"
            
  connections:
    - from: stellar-data-lake
      to: xdr-to-arrow
    - from: xdr-to-arrow
      to: ttp-extractor
    - from: xdr-to-arrow
      to: parquet-storage
    - from: ttp-extractor
      to: parquet-storage
```

### TTP-Processor-Demo with Arrow Support

```yaml
# ttp-processor config
arrow_mode:
  enabled: true
  batch_size: 5000
  flush_interval: "60s"
  flight_server:
    address: ":8815"
    tls_enabled: false
  parquet_archiver:
    enabled: true
    output_path: "gs://obsrvr-ttp-events/parquet"
    partition_by: "daily"
    compression: "zstd"
    iceberg_enabled: true

# Maintain existing streaming config
streaming_mode:
  enabled: true
  grpc_server:
    address: ":8080"
  filters:
    accounts: []
    contracts: []
```

## Benefits and Use Cases

### Performance Benefits

1. **Storage Compression**: Parquet's columnar compression reduces storage costs by 60-80%
2. **Query Performance**: Columnar scanning improves analytical query performance by 10-100x
3. **Network Efficiency**: Arrow Flight reduces serialization overhead by 50-70%
4. **Memory Efficiency**: Vectorized processing reduces memory allocations

### Use Cases Enabled

1. **Real-time Analytics**: Sub-second queries over recent blockchain data
2. **Historical Analysis**: Efficient processing of years of blockchain history
3. **Machine Learning**: Feature extraction for fraud detection, trading algorithms
4. **Data Warehousing**: Integration with modern analytics stacks (Snowflake, BigQuery)
5. **Cross-chain Analysis**: Standardized format for multi-blockchain analytics

### Cost Optimization

1. **Storage**: 70% reduction in cloud storage costs through compression
2. **Compute**: 50% reduction in query costs through columnar efficiency  
3. **Network**: 60% reduction in data transfer through Arrow Flight
4. **Operational**: Simplified data pipeline architecture

This comprehensive upgrade plan maintains the real-time capabilities of existing systems while adding high-performance columnar processing for analytics workloads. The phased approach ensures minimal disruption to existing services while providing a clear path to modern data architecture.