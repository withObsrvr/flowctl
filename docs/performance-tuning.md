# Performance Tuning Guide

This guide covers strategies for optimizing flowctl pipeline performance, identifying bottlenecks, and scaling for high throughput.

## Table of Contents

- [Performance Overview](#performance-overview)
- [Profiling and Monitoring](#profiling-and-monitoring)
- [Component-Level Optimization](#component-level-optimization)
- [Pipeline Topology Optimization](#pipeline-topology-optimization)
- [Resource Management](#resource-management)
- [Scaling Strategies](#scaling-strategies)
- [Troubleshooting Performance Issues](#troubleshooting-performance-issues)

## Performance Overview

### Typical Performance Characteristics

```
Pipeline Configuration    Throughput          Latency
─────────────────────────────────────────────────────────────
Simple Linear            10K-100K events/s   < 10ms end-to-end
(Source→Processor→Sink)

Fan-Out (1 source,       Limited by          < 20ms
multiple sinks)          slowest sink        (+ sink latency)

Fan-In (multiple         Sum of sources      < 10ms
sources, 1 sink)         (if sink can keep up) (+ processing)

Complex DAG              Varies significantly Varies
                         (profile required)   (profile required)
```

### Performance Factors

1. **Component Processing Speed**: How fast components transform data
2. **I/O Performance**: Database writes, API calls, disk operations
3. **Network Latency**: gRPC communication between components
4. **Serialization**: Protobuf encoding/decoding overhead
5. **Backpressure**: Slow components blocking faster ones
6. **Resource Limits**: CPU, memory, network bandwidth

## Profiling and Monitoring

### Enable Metrics Collection

flowctl components built with flowctl-sdk automatically expose metrics:

```yaml
sources:
  - id: my-source
    command: ["/path/to/my-source"]
    env:
      ENABLE_METRICS: "true"
      METRICS_PORT: "9090"  # Prometheus metrics endpoint
```

Access metrics:
```bash
curl http://localhost:9090/metrics
```

### Key Metrics to Monitor

#### Component Metrics

```
# Events processed per second
component_events_processed_total{component_id="source-1"}

# Processing latency (histogram)
component_processing_duration_seconds{component_id="processor-1"}

# Current queue depth
component_queue_depth{component_id="sink-1"}

# Error rate
component_errors_total{component_id="processor-1"}
```

#### Control Plane Metrics

```
# Registered components
flowctl_registered_components

# Healthy components
flowctl_healthy_components

# Active streams
flowctl_active_streams
```

### Profiling with pprof

For Go-based components:

```go
import _ "net/http/pprof"

func main() {
    go func() {
        http.ListenAndServe("localhost:6060", nil)
    }()

    // ... component code
}
```

Profile the component:
```bash
# CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Goroutine profile
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Identifying Bottlenecks

```bash
# 1. Enable debug logging
./bin/flowctl run pipeline.yaml --log-level=debug

# 2. Monitor component queue depths
# High queue depth indicates backpressure

# 3. Check processing times
# Look for "processing took Xms" in logs

# 4. Identify slow component
# The component with highest latency or lowest throughput is likely the bottleneck
```

## Component-Level Optimization

### Source Optimization

#### 1. Batch Data Fetching

❌ **Inefficient (fetch one at a time):**
```go
func produceData(ctx context.Context) ([]*flowpb.Event, error) {
    event := fetchSingleEvent()  // One API call per event
    return []*flowpb.Event{event}, nil
}
```

✅ **Efficient (batch fetch):**
```go
func produceData(ctx context.Context) ([]*flowpb.Event, error) {
    events := fetchBatchEvents(100)  // One API call for 100 events
    return events, nil
}
```

**Impact:** Reduces API overhead, increases throughput 10-100x

#### 2. Connection Pooling

```go
// Initialize once
var dbPool *sql.DB

func init() {
    dbPool, _ = sql.Open("postgres", connStr)
    dbPool.SetMaxOpenConns(25)
    dbPool.SetMaxIdleConns(5)
}

func produceData(ctx context.Context) ([]*flowpb.Event, error) {
    // Reuse pooled connection
    rows, _ := dbPool.QueryContext(ctx, "SELECT ...")
    // ...
}
```

**Impact:** Eliminates connection overhead, increases throughput 5-10x

#### 3. Async I/O

```go
func produceData(ctx context.Context) ([]*flowpb.Event, error) {
    // Use non-blocking I/O
    results := make(chan *flowpb.Event, 100)

    go func() {
        for data := range dataChannel {
            results <- processAsync(data)
        }
    }()

    // ...
}
```

### Processor Optimization

#### 1. Minimize Allocations

❌ **Inefficient (many allocations):**
```go
func processEvent(ctx context.Context, event *flowpb.Event) ([]*flowpb.Event, error) {
    // Creates new map every time
    data := make(map[string]interface{})
    json.Unmarshal(event.Data, &data)

    result := make(map[string]interface{})
    result["processed"] = true
    // ...
}
```

✅ **Efficient (reuse buffers):**
```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make(map[string]interface{}, 10)
    },
}

func processEvent(ctx context.Context, event *flowpb.Event) ([]*flowpb.Event, error) {
    data := bufferPool.Get().(map[string]interface{})
    defer bufferPool.Put(data)

    // Reuse existing map
    json.Unmarshal(event.Data, &data)
    // ...
}
```

**Impact:** Reduces GC pressure, improves latency 2-5x

#### 2. Avoid Expensive Operations in Hot Path

❌ **Inefficient:**
```go
func processEvent(ctx context.Context, event *flowpb.Event) ([]*flowpb.Event, error) {
    // Compiling regex every time!
    re := regexp.MustCompile(`pattern`)
    if re.Match(event.Data) {
        // ...
    }
}
```

✅ **Efficient:**
```go
var re = regexp.MustCompile(`pattern`)  // Compile once

func processEvent(ctx context.Context, event *flowpb.Event) ([]*flowpb.Event, error) {
    if re.Match(event.Data) {
        // ...
    }
}
```

#### 3. Parallel Processing

```go
func processEvent(ctx context.Context, event *flowpb.Event) ([]*flowpb.Event, error) {
    // If processing is CPU-intensive and independent
    results := make([]*flowpb.Event, len(events))

    var wg sync.WaitGroup
    for i, evt := range events {
        wg.Add(1)
        go func(idx int, e *flowpb.Event) {
            defer wg.Done()
            results[idx] = processOne(e)
        }(i, evt)
    }
    wg.Wait()

    return results, nil
}
```

**Impact:** Utilizes multiple CPU cores, increases throughput 2-4x

### Sink Optimization

#### 1. Batch Writes

❌ **Inefficient (one write per event):**
```go
func consumeEvent(ctx context.Context, event *flowpb.Event) error {
    // One database INSERT per event
    _, err := db.ExecContext(ctx, "INSERT INTO ...", event.Data)
    return err
}
```

✅ **Efficient (batch writes):**
```go
var (
    eventBatch = make([]*flowpb.Event, 0, 100)
    batchMutex sync.Mutex
)

func consumeEvent(ctx context.Context, event *flowpb.Event) error {
    batchMutex.Lock()
    eventBatch = append(eventBatch, event)
    shouldFlush := len(eventBatch) >= 100
    batchMutex.Unlock()

    if shouldFlush {
        return flushBatch()
    }
    return nil
}

func flushBatch() error {
    // One database query for 100 events
    // INSERT INTO ... VALUES ($1,$2), ($3,$4), ...
}
```

**Impact:** Reduces database round-trips, increases throughput 10-50x

#### 2. Async Writes with Buffering

```go
type AsyncSink struct {
    writeQueue chan *flowpb.Event
}

func (s *AsyncSink) consumeEvent(ctx context.Context, event *flowpb.Event) error {
    select {
    case s.writeQueue <- event:
        return nil  // Queued, return immediately
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (s *AsyncSink) writerWorker() {
    batch := make([]*flowpb.Event, 0, 100)
    ticker := time.NewTicker(100 * time.Millisecond)

    for {
        select {
        case event := <-s.writeQueue:
            batch = append(batch, event)
            if len(batch) >= 100 {
                s.flushBatch(batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                s.flushBatch(batch)
                batch = batch[:0]
            }
        }
    }
}
```

**Impact:** Hides I/O latency, improves overall throughput

#### 3. Connection Pooling (Sinks)

```go
var dbPool *sql.DB

func init() {
    dbPool, _ = sql.Open("postgres", connStr)
    // Tune pool size based on load
    dbPool.SetMaxOpenConns(50)    // Adjust based on DB capacity
    dbPool.SetMaxIdleConns(10)
    dbPool.SetConnMaxLifetime(5 * time.Minute)
}
```

## Pipeline Topology Optimization

### Pattern 1: Reduce Serialization Overhead

❌ **Inefficient (many hops):**
```
Source → Processor1 → Processor2 → Processor3 → Sink
         (serialize)  (serialize)  (serialize)
```

Each hop requires serialization/deserialization overhead.

✅ **Efficient (combined processing):**
```
Source → CombinedProcessor → Sink
         (all logic here)
```

**When to use:** If processors are simple and latency is critical.

### Pattern 2: Parallel Processing (Fan-Out + Fan-In)

```yaml
spec:
  sources:
    - id: source
      command: [...]

  processors:
    # Split work across multiple processors
    - id: processor-1
      command: [...]
      inputs: ["source"]
      env:
        PARTITION: "0"  # Process partition 0

    - id: processor-2
      command: [...]
      inputs: ["source"]
      env:
        PARTITION: "1"  # Process partition 1

    - id: processor-3
      command: [...]
      inputs: ["source"]
      env:
        PARTITION: "2"  # Process partition 2

  sinks:
    - id: sink
      command: [...]
      inputs: ["processor-1", "processor-2", "processor-3"]
```

**Impact:** Distributes CPU load, increases throughput 2-3x

### Pattern 3: Asynchronous Sinks

```
Source → Processor ─┬→ Fast Sink (file)
                    ├→ Slow Sink 1 (DB, async)
                    └→ Slow Sink 2 (API, async)
```

Fast operations don't wait for slow ones.

## Resource Management

### CPU Optimization

#### 1. Process Affinity (Linux)

```bash
# Pin component to specific CPU cores
taskset -c 0,1 /path/to/source-binary &
taskset -c 2,3 /path/to/processor-binary &
taskset -c 4,5 /path/to/sink-binary &
```

**Impact:** Reduces context switching, improves cache locality

#### 2. Go Runtime Tuning

```go
func main() {
    // Set max CPUs (default is number of cores)
    runtime.GOMAXPROCS(4)  // Use 4 cores

    // ... component code
}
```

### Memory Optimization

#### 1. Limit Queue Sizes

```yaml
sources:
  - id: my-source
    env:
      MAX_QUEUE_SIZE: "1000"  # Prevent unbounded growth
```

#### 2. Set GC Target (Go)

```bash
# Environment variable approach
GOGC=100 /path/to/component  # Default (100% growth before GC)
GOGC=50 /path/to/component   # More aggressive GC (lower memory, higher CPU)
GOGC=200 /path/to/component  # Less aggressive GC (higher memory, lower CPU)
```

#### 3. Memory Limits (Docker/Kubernetes)

```yaml
# Docker driver
spec:
  driver: docker
  sinks:
    - id: my-sink
      image: "ghcr.io/org/sink:v1"
      resources:
        limits:
          memory: "512Mi"  # Hard limit
        requests:
          memory: "256Mi"  # Reserved
```

### Network Optimization

#### 1. Increase gRPC Message Size Limits

```go
// In component code
opts := []grpc.ServerOption{
    grpc.MaxRecvMsgSize(10 * 1024 * 1024),  // 10 MB
    grpc.MaxSendMsgSize(10 * 1024 * 1024),  // 10 MB
}
```

#### 2. Use Connection Pooling

```go
// gRPC client connection pool
var connPool []*grpc.ClientConn

func init() {
    for i := 0; i < 5; i++ {
        conn, _ := grpc.Dial(address, opts...)
        connPool = append(connPool, conn)
    }
}

func getConnection() *grpc.ClientConn {
    return connPool[rand.Intn(len(connPool))]
}
```

#### 3. Enable gRPC Compression

```go
opts := []grpc.DialOption{
    grpc.WithDefaultCallOptions(grpc.UseCompressor("gzip")),
}
```

**Impact:** Reduces network bandwidth, may increase CPU usage

## Scaling Strategies

### Vertical Scaling (Scale Up)

**Increase resources for existing components:**

```yaml
# Kubernetes example
spec:
  driver: kubernetes
  processors:
    - id: heavy-processor
      image: "ghcr.io/org/processor:v1"
      resources:
        requests:
          cpu: "2"        # More CPU
          memory: "4Gi"   # More memory
        limits:
          cpu: "4"
          memory: "8Gi"
```

**When to use:**
- Single-threaded bottleneck
- Memory-intensive operations
- Simpler than horizontal scaling

### Horizontal Scaling (Scale Out)

#### Pattern 1: Replicate Sinks

```yaml
sinks:
  # Multiple instances of same sink
  - id: sink-1
    command: [...]
    inputs: ["processor"]

  - id: sink-2
    command: [...]
    inputs: ["processor"]

  - id: sink-3
    command: [...]
    inputs: ["processor"]
```

**Impact:** Distributes write load, increases throughput 3x

#### Pattern 2: Partition Processing

```yaml
sources:
  # Multiple sources for different data partitions
  - id: source-partition-0
    command: [...]
    env:
      PARTITION_ID: "0"
      PARTITION_COUNT: "4"

  - id: source-partition-1
    command: [...]
    env:
      PARTITION_ID: "1"
      PARTITION_COUNT: "4"

  # ... partitions 2 and 3 ...

processors:
  # Each partition has its own processor
  - id: processor-0
    command: [...]
    inputs: ["source-partition-0"]

  - id: processor-1
    command: [...]
    inputs: ["source-partition-1"]

  # ...
```

**Impact:** Parallelizes entire pipeline

#### Pattern 3: Load Balancing

Use external load balancer to distribute events:

```
        ┌─ Processor Instance 1
Load   ─┼─ Processor Instance 2
Balancer└─ Processor Instance 3
```

Implement in source or processor component.

### Kubernetes Auto-Scaling

```yaml
# HorizontalPodAutoscaler for processor
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: processor-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: processor
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## Troubleshooting Performance Issues

### Issue 1: Low Throughput

**Symptoms:**
- Events processed per second is low
- Expected: 10K/s, actual: 100/s

**Debug:**
```bash
# 1. Check component CPU usage
top -p $(pgrep source-binary)
top -p $(pgrep processor-binary)
top -p $(pgrep sink-binary)

# 2. Check queue depths
# High queue depth = backpressure

# 3. Profile slow component
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

**Common causes:**
- Database writes not batched
- Synchronous I/O blocking
- CPU-bound processing not parallelized
- Network latency

**Solutions:**
- Implement batching
- Use async I/O
- Add parallel processing
- Increase resources

### Issue 2: High Latency

**Symptoms:**
- End-to-end latency > 100ms
- Events delayed

**Debug:**
```bash
# Enable debug logging to see timing
./bin/flowctl run pipeline.yaml --log-level=debug

# Look for:
# "processing took Xms"
# "waiting for downstream"
```

**Common causes:**
- Slow database queries
- API call latency
- Too many pipeline hops
- GC pauses

**Solutions:**
- Add database indexes
- Use connection pooling
- Combine processors
- Tune GC settings

### Issue 3: Memory Growth

**Symptoms:**
- Memory usage increasing over time
- Eventually OOM crashes

**Debug:**
```bash
# Memory profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Check for:
# - Goroutine leaks
# - Unbounded queues
# - Memory leaks in processing
```

**Common causes:**
- Queue unbounded growth
- Goroutine leaks
- Not releasing resources

**Solutions:**
- Set MAX_QUEUE_SIZE
- Ensure proper cleanup
- Fix goroutine leaks

### Issue 4: Backpressure Buildup

**Symptoms:**
- Source slowing down
- Events queuing up
- High memory usage

**Debug:**
```bash
# Check queue depths
# If sink queue is full → sink is bottleneck

# Check sink performance
curl http://localhost:8090/metrics | grep processing_duration
```

**Solutions:**
- Optimize sink (batching, async)
- Add more sink instances
- Reduce event size
- Increase sink resources

## Performance Checklist

### Component Level
- [ ] Batch API calls and database queries
- [ ] Use connection pooling
- [ ] Implement async I/O where appropriate
- [ ] Minimize allocations in hot path
- [ ] Reuse buffers with sync.Pool
- [ ] Batch writes in sinks
- [ ] Profile CPU and memory usage

### Pipeline Level
- [ ] Minimize number of hops (serialization overhead)
- [ ] Use fan-out for parallelism where applicable
- [ ] Ensure components don't block each other
- [ ] Use appropriate topology for workload
- [ ] Consider partitioning for horizontal scaling

### Resource Level
- [ ] Set appropriate memory limits
- [ ] Tune GOMAXPROCS for CPU usage
- [ ] Use process affinity if needed
- [ ] Monitor queue depths
- [ ] Set GC target based on workload
- [ ] Use appropriate network buffer sizes

### Monitoring
- [ ] Enable metrics collection
- [ ] Monitor throughput per component
- [ ] Track processing latency
- [ ] Watch for backpressure signals
- [ ] Alert on error rates
- [ ] Profile regularly

## Benchmarking

### Simple Throughput Test

```bash
# Run pipeline with test data
./bin/flowctl run pipeline.yaml

# Count events processed
# Check sink output or metrics

# Calculate throughput
# events_processed / elapsed_seconds = events/sec
```

### Load Testing

```yaml
# Create load test source
sources:
  - id: load-generator
    command: ["/path/to/load-generator"]
    env:
      EVENTS_PER_SECOND: "10000"
      DURATION_SECONDS: "60"
      EVENT_SIZE_BYTES: "1024"
```

Monitor:
- Throughput sustained
- Latency percentiles (p50, p95, p99)
- Resource usage (CPU, memory)
- Error rates

## Additional Resources

- **Architecture**: [architecture.md](architecture.md) - System design details
- **Configuration**: [configuration.md](configuration.md) - Pipeline configuration
- **Building Components**: [building-components.md](building-components.md) - Component development
- **Monitoring**: Prometheus metrics and Grafana dashboards

## Questions?

- **GitHub Issues**: https://github.com/withobsrvr/flowctl/issues
- **Documentation**: https://github.com/withobsrvr/flowctl
