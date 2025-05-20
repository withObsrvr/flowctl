# DAG-Based Processing Pipeline

flowctl now supports a Directed Acyclic Graph (DAG) based processing pipeline, which provides several advantages over the simple linear pipeline:

1. **Flexible Processor Chaining**: Connect processors in complex topologies, not just linear chains
2. **Parallel Processing**: Process data through multiple processors simultaneously
3. **Buffered Channels**: Manage backpressure with buffered Go channels between processors
4. **Fan-out/Fan-in**: Support for splitting and merging data flows
5. **Strong Typing**: Better type safety with explicit event type declarations

## Overview

The DAG-based pipeline allows you to:

- Connect sources to multiple processors
- Connect processors to multiple downstream processors
- Connect processors to multiple sinks
- Specify exact event types for each connection
- Buffer messages between components
- Process data asynchronously and in parallel

## Configuration

To use the DAG-based pipeline, set the environment variable:

```sh
export FLOWCTL_PIPELINE_TYPE=dag
```

The default pipeline type is now `dag`, so this is only necessary if you want to explicitly set it. To use the legacy simple pipeline:

```sh
export FLOWCTL_PIPELINE_TYPE=simple
```

## YAML Pipeline Configuration

When using the DAG-based pipeline, you need to provide event type information in your pipeline YAML configuration:

```yaml
apiVersion: flowctl/v1
kind: Pipeline
metadata:
  name: example-pipeline
spec:
  sources:
    - id: stellar-source
      # ... other source configuration ...
      output_event_types: ["stellar.ledger", "stellar.transaction"]
  
  processors:
    - id: token-transfer-processor
      # ... other processor configuration ...
      inputs: ["stellar-source"]
      input_event_types: ["stellar.transaction"]
      output_event_types: ["token.transfer"]
        
    - id: account-processor
      # ... other processor configuration ...
      inputs: ["stellar-source"]
      input_event_types: ["stellar.transaction"]
      output_event_types: ["account.activity"]
        
    - id: alert-processor
      # ... other processor configuration ...
      inputs: ["token-transfer-processor", "account-processor"]
      input_event_types: ["token.transfer", "account.activity"]
      output_event_types: ["alert.notification"]
  
  sinks:
    - id: kafka-sink
      # ... other sink configuration ...
      inputs: ["token-transfer-processor", "account-processor", "alert-processor"]
```

## Flow Control and Backpressure

The DAG-based pipeline uses buffered channels to handle flow control and backpressure:

1. Each connection between components uses a buffered channel
2. When a channel's buffer fills up, upstream components block until space is available
3. This provides natural backpressure in the system
4. Buffer sizes can be configured to optimize performance

## Example DAG Structure

Here's a visualization of an example DAG pipeline:

```
              ┌─────────────────┐
              │  stellar-source │
              └─────────────────┘
                     │
                     ▼
       ┌───────────────────────────┐
       │                           │
       ▼                           ▼
┌─────────────────┐     ┌─────────────────┐
│ token-processor │     │account-processor│
└─────────────────┘     └─────────────────┘
       │                           │
       │                           │
       │         ┌─────────────────┘
       │         │
       ▼         ▼
┌─────────────────┐
│ alert-processor │
└─────────────────┘
       │
       ▼
┌─────────────────┐
│   kafka-sink    │
└─────────────────┘
```

In this example:
- The stellar source produces events
- Events are processed by both the token processor and account processor in parallel
- The alert processor combines outputs from both processors
- The kafka sink receives alerts

## Usage in Code

If you're building custom processors, you can leverage the DAG API directly:

```go
// Create a DAG with buffer size
dag := core.NewDAG(1000)

// Add sources, processors, and sinks
dag.AddSource("source1", mySource)
dag.AddProcessor("proc1", myProcessor, []string{"event.type1"})
dag.AddProcessor("proc2", myProcessor2, []string{"event.type2"})
dag.AddSinkChannel("sink1")

// Connect components
dag.ConnectSourceToProcessor("source1", "proc1", "event.type1")
dag.Connect("proc1", "proc2", "event.type2")
dag.ConnectProcessorToSink("proc2", "sink1", "event.type3")

// Start the DAG
ctx := context.Background()
dag.Start(ctx)

// Stop when done
dag.Stop()
```

## Further Reading

For more information, see:
- [Example DAG Pipeline Configuration](../examples/dag-pipeline.yaml)
- [pipeline_dag.go](../internal/core/pipeline_dag.go)
- [dag.go](../internal/core/dag.go)