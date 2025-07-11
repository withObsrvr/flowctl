# Local Process Pipeline Examples

This directory contains example pipelines designed for local process execution with the embedded control plane.

## Examples

### 1. Hello World Pipeline (`hello-world-pipeline.yaml`)
**Purpose**: Simplest possible pipeline to test embedded control plane
**Components**: echo → tr → cat
**Usage**:
```bash
./bin/flowctl run examples/hello-world-pipeline.yaml
```

### 2. Simple Local Pipeline (`simple-local-pipeline.yaml`)
**Purpose**: Demonstrates longer-running processes with multiple sources
**Components**: 
- Sources: date generator, counter
- Processor: sed prefix adder
- Sink: console output
**Usage**:
```bash
./bin/flowctl run examples/simple-local-pipeline.yaml
```

### 3. Mock Component Pipeline (`mock-pipeline.yaml`)
**Purpose**: Uses mock components that simulate real flowctl SDK integration
**Components**: All use `mock-component.sh` script
**Usage**:
```bash
./bin/flowctl run examples/mock-pipeline.yaml
```

### 4. Local Process Pipeline (`local-process-pipeline.yaml`)
**Purpose**: More complex pipeline with file I/O and JSON processing
**Components**: file sources, processors (tr, jq), file sinks
**Usage**:
```bash
# Create test input file
echo "hello world" > /tmp/test-input.txt

# Run pipeline
./bin/flowctl run examples/local-process-pipeline.yaml

# Check outputs
cat /tmp/pipeline-output.txt
cat /tmp/pipeline-results.log
```

### 5. Local Test Pipeline (`local-test.yaml`)
**Purpose**: Real Stellar blockchain pipeline converted to new format
**Components**: 
- Source: stellar-live-source-datalake
- Processor: ttp-processor
- Sink: consumer-app
**Usage**:
```bash
# Note: Requires actual component binaries
./bin/flowctl run examples/local-test.yaml
```

### 6. Long Running Pipeline (`long-running-pipeline.yaml`)
**Purpose**: Demonstrates monitoring capabilities
**Components**: Timer source → counter processor → logger sink
**Usage**:
```bash
./bin/flowctl run examples/long-running-pipeline.yaml
```

## Key Differences from Docker Pipelines

### Local Process Pipelines
- Use `command` and `args` instead of `image`
- Execute binaries directly on the host
- Share the same filesystem
- Use Unix pipes for communication
- Environment variables for configuration

### Docker Pipelines
- Use `image` to specify container images
- Isolated execution environments
- Volume mounts for file sharing
- Network communication between containers
- Container orchestration

## Pipeline Structure for Local Processes

```yaml
apiVersion: v1
kind: Pipeline
metadata:
  name: my-local-pipeline
spec:
  sources:
    - id: my-source
      type: my-source-type
      command: ["./my-binary"]          # Binary to execute
      args: ["--option", "value"]       # Command arguments
      env:                              # Environment variables
        FLOWCTL_SERVICE_TYPE: "source"
      outputEventTypes:
        - my.event
  
  processors:
    - id: my-processor
      type: my-processor-type
      command: ["./my-processor"]
      inputs:
        - my-source                     # Input dependencies
      inputEventTypes:
        - my.event
      outputEventTypes:
        - my.processed
      env:
        FLOWCTL_SERVICE_TYPE: "processor"
  
  sinks:
    - id: my-sink
      type: my-sink-type
      command: ["./my-sink"]
      inputs:
        - my-processor
      inputEventTypes:
        - my.processed
      env:
        FLOWCTL_SERVICE_TYPE: "sink"
```

## Environment Variables

The embedded control plane automatically sets these environment variables for each component:

```bash
ENABLE_FLOWCTL=true                    # Enables flowctl integration
FLOWCTL_ENDPOINT=http://localhost:8080 # Control plane endpoint
FLOWCTL_HEARTBEAT_INTERVAL=10s         # Heartbeat frequency
FLOWCTL_SERVICE_ID=<component-id>      # Unique component identifier
```

## Testing the Examples

### 1. Basic Test
```bash
# Test embedded control plane startup
./bin/flowctl run examples/hello-world-pipeline.yaml --log-level debug
```

### 2. Monitor Components
```bash
# In terminal 1: Start pipeline
./bin/flowctl run examples/simple-local-pipeline.yaml

# In terminal 2: Monitor components
./bin/flowctl list
```

### 3. Test Error Handling
```bash
# Try to run non-existent pipeline
./bin/flowctl run examples/nonexistent.yaml
```

## Expected Behavior

1. **Control Plane Startup**: Embedded control plane starts on port 8080
2. **Component Startup**: Components start in dependency order (sources → processors → sinks)
3. **Registration**: Components register with control plane (if they have flowctl SDK)
4. **Monitoring**: Real-time status updates show component health
5. **Graceful Shutdown**: Ctrl+C stops all components and control plane

## Troubleshooting

### Common Issues

1. **"no such file or directory"**: Binary doesn't exist
   - Check that the command path is correct
   - Ensure binary is executable (`chmod +x`)

2. **"connection refused"**: Control plane not running
   - Make sure `flowctl run` is active
   - Check port 8080 is available

3. **Component doesn't register**: Missing flowctl SDK integration
   - Real components need flowctl SDK to register
   - Mock components simulate registration

### Debug Mode
```bash
./bin/flowctl run examples/hello-world-pipeline.yaml --log-level debug --show-status
```

This shows detailed startup logs and component status monitoring.

## Next Steps

1. **Real Components**: Replace mock components with actual binaries
2. **flowctl SDK**: Integrate components with flowctl SDK for registration
3. **Container Support**: Use `--orchestrator container` for Docker execution
4. **Production**: Use persistent storage and TLS for production deployments