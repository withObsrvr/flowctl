# flowctl Architecture

This document provides a comprehensive overview of flowctl's architecture, design patterns, and internal components.

## Table of Contents

- [High-Level Architecture](#high-level-architecture)
- [Component Model](#component-model)
- [Control Plane](#control-plane)
- [Data Flow](#data-flow)
- [Execution Models](#execution-models)
- [Communication Patterns](#communication-patterns)
- [State Management](#state-management)
- [Design Principles](#design-principles)

## High-Level Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         flowctl CLI                              │
│  • Command Line Interface (Cobra)                               │
│  • Configuration Loading (Viper)                                │
│  • Pipeline Validation (CUE Schema)                             │
└────────────────────┬────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Control Plane (Embedded)                      │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Component Registry (gRPC API)                          │   │
│  │  • Registration Service                                 │   │
│  │  • Health Check Manager                                 │   │
│  │  • Heartbeat Monitor                                    │   │
│  │  • Stream Orchestrator                                  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Storage Layer                                          │   │
│  │  • BoltDB (persistent) or In-Memory                     │   │
│  │  • Component metadata                                   │   │
│  │  • Pipeline state                                       │   │
│  └─────────────────────────────────────────────────────────┘   │
└────────────┬────────────┬────────────┬────────────┬─────────────┘
             │            │            │            │
             ▼            ▼            ▼            ▼
     ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
     │   Source     │ │  Processor   │ │    Sink      │
     │  Component   │ │  Component   │ │  Component   │
     │              │ │              │ │              │
     │ • Produces   │ │ • Transforms │ │ • Consumes   │
     │   data       │ │   data       │ │   data       │
     │ • Registers  │ │ • Registers  │ │ • Registers  │
     │ • Heartbeat  │ │ • Heartbeat  │ │ • Heartbeat  │
     └──────┬───────┘ └──────┬───────┘ └──────┬───────┘
            │                │                │
            │  gRPC Stream   │  gRPC Stream   │
            └───────┬────────┴────────┬───────┘
                    │                 │
                    ▼                 ▼
            ┌─────────────────────────────────┐
            │     Data Flow Pipeline          │
            │  (Protobuf Events via gRPC)     │
            └─────────────────────────────────┘
```

### Key Components

1. **flowctl CLI**: User-facing command-line interface
2. **Control Plane**: Embedded orchestration service
3. **Components**: Separate processes (sources, processors, sinks)
4. **Storage Layer**: Persistent or in-memory state
5. **Communication Layer**: gRPC for all inter-component communication

## Component Model

### Component Lifecycle

```
┌─────────────┐
│   Start     │
│ Component   │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────┐
│  1. Initialize                  │
│     • Load configuration        │
│     • Setup gRPC server         │
│     • Prepare resources         │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│  2. Register with Control Plane │
│     • Send registration request │
│     • Provide metadata          │
│     • Get component ID          │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│  3. Start Health Checks         │
│     • HTTP endpoint: /health    │
│     • Regular heartbeats        │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│  4. Process Data                │
│     • Source: Produce events    │
│     • Processor: Transform      │
│     • Sink: Consume events      │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│  5. Graceful Shutdown           │
│     • Stop accepting new data   │
│     • Flush pending events      │
│     • Close connections         │
│     • Deregister                │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────┐
│     End     │
└─────────────┘
```

### Component Types

#### Source Component

```
┌──────────────────────────────────────────────┐
│              Source Component                 │
├──────────────────────────────────────────────┤
│                                              │
│  ┌────────────────────────────────────┐    │
│  │   Data Producer                    │    │
│  │   • Fetch from external system     │    │
│  │   • Poll API, read database, etc.  │    │
│  │   • Generate Protobuf events       │    │
│  └────────────────┬───────────────────┘    │
│                   │                         │
│                   ▼                         │
│  ┌────────────────────────────────────┐    │
│  │   gRPC Server                      │    │
│  │   • Stream events downstream       │    │
│  │   • Handle backpressure            │    │
│  └────────────────┬───────────────────┘    │
│                   │                         │
│                   ▼                         │
│  ┌────────────────────────────────────┐    │
│  │   Health & Registration            │    │
│  │   • /health endpoint               │    │
│  │   • Heartbeat to control plane     │    │
│  └────────────────────────────────────┘    │
│                                              │
└──────────────────┬───────────────────────────┘
                   │
                   │ Protobuf Events
                   ▼
           [Downstream Components]
```

#### Processor Component

```
           [Upstream Components]
                   │
                   │ Protobuf Events
                   ▼
┌──────────────────────────────────────────────┐
│            Processor Component                │
├──────────────────────────────────────────────┤
│                                              │
│  ┌────────────────────────────────────┐    │
│  │   gRPC Client                      │    │
│  │   • Receive events from upstream   │    │
│  │   • Multiple inputs supported      │    │
│  └────────────────┬───────────────────┘    │
│                   │                         │
│                   ▼                         │
│  ┌────────────────────────────────────┐    │
│  │   Data Transformer                 │    │
│  │   • Process event                  │    │
│  │   • Filter, enrich, aggregate      │    │
│  │   • Generate output events         │    │
│  └────────────────┬───────────────────┘    │
│                   │                         │
│                   ▼                         │
│  ┌────────────────────────────────────┐    │
│  │   gRPC Server                      │    │
│  │   • Stream events downstream       │    │
│  │   • Handle backpressure            │    │
│  └────────────────┬───────────────────┘    │
│                   │                         │
│                   ▼                         │
│  ┌────────────────────────────────────┐    │
│  │   Health & Registration            │    │
│  │   • /health endpoint               │    │
│  │   • Heartbeat to control plane     │    │
│  └────────────────────────────────────┘    │
│                                              │
└──────────────────┬───────────────────────────┘
                   │
                   │ Protobuf Events
                   ▼
           [Downstream Components]
```

#### Sink Component

```
           [Upstream Components]
                   │
                   │ Protobuf Events
                   ▼
┌──────────────────────────────────────────────┐
│              Sink Component                   │
├──────────────────────────────────────────────┤
│                                              │
│  ┌────────────────────────────────────┐    │
│  │   gRPC Client                      │    │
│  │   • Receive events from upstream   │    │
│  │   • Multiple inputs supported      │    │
│  └────────────────┬───────────────────┘    │
│                   │                         │
│                   ▼                         │
│  ┌────────────────────────────────────┐    │
│  │   Data Consumer                    │    │
│  │   • Process event                  │    │
│  │   • Write to storage/external      │    │
│  │   • Batch operations               │    │
│  └────────────────┬───────────────────┘    │
│                   │                         │
│                   ▼                         │
│  ┌────────────────────────────────────┐    │
│  │   External System                  │    │
│  │   • PostgreSQL, Kafka, etc.        │    │
│  │   • Webhooks, APIs                 │    │
│  │   • File systems                   │    │
│  └────────────────┬───────────────────┘    │
│                   │                         │
│                   ▼                         │
│  ┌────────────────────────────────────┐    │
│  │   Health & Registration            │    │
│  │   • /health endpoint               │    │
│  │   • Heartbeat to control plane     │    │
│  └────────────────────────────────────┘    │
│                                              │
└──────────────────────────────────────────────┘
```

## Control Plane

### Control Plane Architecture

```
┌────────────────────────────────────────────────────────────┐
│                   Control Plane (Port 8080)                 │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  ┌──────────────────────────────────────────────────┐    │
│  │           gRPC API Server                        │    │
│  │                                                  │    │
│  │  Service: ComponentRegistry                     │    │
│  │  • RegisterComponent(request)                   │    │
│  │  • Heartbeat(component_id)                      │    │
│  │  • GetComponents()                              │    │
│  │  • GetComponentStatus(id)                       │    │
│  └──────────────────┬───────────────────────────────┘    │
│                     │                                     │
│                     ▼                                     │
│  ┌──────────────────────────────────────────────────┐    │
│  │       Component Registry (In-Memory)             │    │
│  │                                                  │    │
│  │  Components Map:                                │    │
│  │  {                                              │    │
│  │    "source-1": {                                │    │
│  │      id: "source-1",                            │    │
│  │      type: "source",                            │    │
│  │      status: "healthy",                         │    │
│  │      last_heartbeat: 1234567890,                │    │
│  │      metadata: {...}                            │    │
│  │    },                                           │    │
│  │    "processor-1": {...},                        │    │
│  │    "sink-1": {...}                              │    │
│  │  }                                              │    │
│  └──────────────────┬───────────────────────────────┘    │
│                     │                                     │
│                     ▼                                     │
│  ┌──────────────────────────────────────────────────┐    │
│  │          Health Monitor                          │    │
│  │                                                  │    │
│  │  • Periodic health checks (every 5s)            │    │
│  │  • Mark unhealthy if no heartbeat (>30s)        │    │
│  │  • Notify on status changes                     │    │
│  └──────────────────┬───────────────────────────────┘    │
│                     │                                     │
│                     ▼                                     │
│  ┌──────────────────────────────────────────────────┐    │
│  │          Stream Orchestrator                     │    │
│  │                                                  │    │
│  │  • Maintain component topology graph            │    │
│  │  • Route events based on inputs config          │    │
│  │  • Handle backpressure                          │    │
│  └──────────────────────────────────────────────────┘    │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

### Registration Flow

```
Component                     Control Plane
    │                              │
    │  1. RegisterComponent        │
    │  ───────────────────────────>│
    │     {                        │
    │       id: "my-source",       │
    │       type: "source",        │
    │       address: "localhost:50051",
    │       metadata: {...}        │
    │     }                        │
    │                              │
    │  2. Registration Response    │
    │  <───────────────────────────│
    │     {                        │
    │       success: true,         │
    │       component_id: "..."    │
    │     }                        │
    │                              │
    │  3. Periodic Heartbeats      │
    │  ───────────────────────────>│
    │     Every 10 seconds         │
    │                              │
    │  4. Heartbeat ACK            │
    │  <───────────────────────────│
    │                              │
    │                              │
    │  (Component processing...)   │
    │                              │
    │                              │
    │  5. Deregister (on shutdown) │
    │  ───────────────────────────>│
    │                              │
```

### Health Monitoring

```
Time: 0s              10s             20s             30s             40s
│                     │               │               │               │
├─ Component Start ───┤               │               │               │
│                     │               │               │               │
├─ Register ──────────┼───────────────┤               │               │
│                     │               │               │               │
├─ Heartbeat ─────────┼───────────────┼───────────────┤               │
│  Status: Healthy    │  Healthy      │  Healthy      │               │
│                     │               │               │               │
│                     │               │               │               │
│  (Component crashes)                │               │               │
│                     │               │               │               │
│                     │               │  X No beat    │               │
│                     │               │  Still OK     │               │
│                     │               │               │               │
│                     │               │               │  X No beat    │
│                     │               │               │  > 30s        │
│                     │               │               │  Mark UNHEALTHY
│                     │               │               │               │
```

## Data Flow

### Event Flow (Simple Pipeline)

```
┌─────────────┐        ┌─────────────┐        ┌─────────────┐
│   Source    │        │  Processor  │        │    Sink     │
│             │        │             │        │             │
│  Produces   │        │ Transforms  │        │  Consumes   │
│  Events     │        │   Events    │        │   Events    │
└──────┬──────┘        └──────┬──────┘        └──────┬──────┘
       │                      │                      │
       │ Event Stream         │ Event Stream         │
       │ (gRPC)              │ (gRPC)              │
       │                      │                      │
       ▼                      ▼                      ▼
  ┌─────────┐           ┌─────────┐           ┌─────────┐
  │ Event 1 │──────────>│ Event 1'│──────────>│ Event 1'│
  │         │           │ (transformed)       │ (stored)│
  └─────────┘           └─────────┘           └─────────┘
       │                      │                      │
       ▼                      ▼                      ▼
  ┌─────────┐           ┌─────────┐           ┌─────────┐
  │ Event 2 │──────────>│ Event 2'│──────────>│ Event 2'│
  └─────────┘           └─────────┘           └─────────┘
```

### Event Flow (Fan-Out)

```
                       ┌─────────────┐
                       │   Source    │
                       │             │
                       │  Produces   │
                       │   Events    │
                       └──────┬──────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
       ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
       │   Sink 1    │ │   Sink 2    │ │   Sink 3    │
       │             │ │             │ │             │
       │ PostgreSQL  │ │  Webhook    │ │    File     │
       └─────────────┘ └─────────────┘ └─────────────┘

Same event sent to all three sinks concurrently
```

### Event Flow (Fan-In)

```
       ┌─────────────┐        ┌─────────────┐
       │  Source 1   │        │  Source 2   │
       │             │        │             │
       │   Stellar   │        │    Kafka    │
       └──────┬──────┘        └──────┬──────┘
              │                      │
              └──────────┬───────────┘
                         │
                         ▼
                  ┌─────────────┐
                  │  Processor  │
                  │             │
                  │   Merges    │
                  └──────┬──────┘
                         │
                         ▼
                  ┌─────────────┐
                  │    Sink     │
                  └─────────────┘

Events from both sources processed together
```

### Event Flow (DAG - Diamond Pattern)

```
                    ┌─────────────┐
                    │   Source    │
                    └──────┬──────┘
                           │
              ┌────────────┴────────────┐
              │                         │
              ▼                         ▼
       ┌─────────────┐           ┌─────────────┐
       │ Processor 1 │           │ Processor 2 │
       │             │           │             │
       │   Filter    │           │  Aggregate  │
       └──────┬──────┘           └──────┬──────┘
              │                         │
              └────────────┬────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │    Sink     │
                    │             │
                    │  Combines   │
                    └─────────────┘

Source data split, processed separately, then combined
```

## Execution Models

### Process Driver (Local Execution)

```
Host Machine
├─ flowctl process (PID 1000)
│  ├─ Control Plane (Port 8080)
│  └─ Process Manager
│     ├─ Spawns source-1 (PID 1001)
│     ├─ Spawns processor-1 (PID 1002)
│     └─ Spawns sink-1 (PID 1003)
│
├─ source-1 (PID 1001)
│  └─ Listens on localhost:50051
│
├─ processor-1 (PID 1002)
│  └─ Listens on localhost:50052
│
└─ sink-1 (PID 1003)
   └─ Listens on localhost:50053

All communication via localhost
Fast startup, simple debugging
```

### Docker Driver (Containerized Execution)

```
Docker Host
├─ flowctl container
│  ├─ Control Plane (Port 8080)
│  └─ Docker API Client
│
├─ source-1 container
│  ├─ Image: ghcr.io/org/source:v1
│  ├─ Network: flowctl-network
│  └─ Port: 50051
│
├─ processor-1 container
│  ├─ Image: ghcr.io/org/processor:v1
│  ├─ Network: flowctl-network
│  └─ Port: 50052
│
└─ sink-1 container
   ├─ Image: ghcr.io/org/sink:v1
   ├─ Network: flowctl-network
   └─ Port: 50053

Isolated environments, easy distribution
```

### Kubernetes Driver (Cluster Execution)

```
Kubernetes Cluster
├─ flowctl Deployment
│  └─ Control Plane Service (ClusterIP)
│
├─ source-1 Pod
│  ├─ Container: ghcr.io/org/source:v1
│  └─ Service: source-1-svc
│
├─ processor-1 Pod
│  ├─ Container: ghcr.io/org/processor:v1
│  └─ Service: processor-1-svc
│
└─ sink-1 Pod
   ├─ Container: ghcr.io/org/sink:v1
   └─ Service: sink-1-svc

Auto-scaling, high availability, service discovery
```

## Communication Patterns

### gRPC Streaming

Components communicate via **bidirectional gRPC streams**:

```
Source                          Processor
  │                                │
  │  Open Stream                   │
  │  ─────────────────────────────>│
  │                                │
  │  Send Event 1                  │
  │  ─────────────────────────────>│
  │                                │
  │  ACK Event 1                   │
  │  <─────────────────────────────│
  │                                │
  │  Send Event 2                  │
  │  ─────────────────────────────>│
  │                                │
  │  ACK Event 2                   │
  │  <─────────────────────────────│
  │                                │
  │  ...continue...                │
  │                                │
```

### Backpressure Handling

```
Fast Source          Slow Sink          Control Plane
     │                   │                    │
     │ Event 1           │                    │
     │ ─────────────────>│                    │
     │                   │ Processing...      │
     │                   │                    │
     │ Event 2           │                    │
     │ ─────────────────>│                    │
     │                   │ Buffer filling...  │
     │                   │                    │
     │ Event 3           │                    │
     │ ─────────────X────│ BACKPRESSURE      │
     │                   │                    │
     │ Wait...           │ Processing...      │
     │                   │                    │
     │                   │ Buffer cleared     │
     │ Event 3 (retry)   │                    │
     │ ─────────────────>│                    │
     │                   │ OK                 │
```

## State Management

### Component State

```
┌──────────────────────────────────────────┐
│          Component Registry              │
├──────────────────────────────────────────┤
│                                          │
│  component_id: "source-1"                │
│  {                                       │
│    id: "source-1",                       │
│    type: "source",                       │
│    status: "healthy",                    │
│    address: "localhost:50051",           │
│    health_endpoint: "localhost:8088",    │
│    last_heartbeat: 1234567890,           │
│    registered_at: 1234567800,            │
│    metadata: {                           │
│      name: "Stellar Live Source",        │
│      version: "1.0.0",                   │
│      output_type: "stellar.ledger.v1"    │
│    },                                    │
│    connections: {                        │
│      downstream: ["processor-1"]         │
│    }                                     │
│  }                                       │
│                                          │
└──────────────────────────────────────────┘
```

### Pipeline State

```
┌──────────────────────────────────────────┐
│            Pipeline State                │
├──────────────────────────────────────────┤
│                                          │
│  pipeline_id: "my-pipeline"              │
│  {                                       │
│    name: "my-pipeline",                  │
│    status: "running",                    │
│    started_at: 1234567800,               │
│    components: [                         │
│      "source-1",                         │
│      "processor-1",                      │
│      "sink-1"                            │
│    ],                                    │
│    topology: {                           │
│      "source-1": {                       │
│        downstream: ["processor-1"]       │
│      },                                  │
│      "processor-1": {                    │
│        upstream: ["source-1"],           │
│        downstream: ["sink-1"]            │
│      },                                  │
│      "sink-1": {                         │
│        upstream: ["processor-1"]         │
│      }                                   │
│    }                                     │
│  }                                       │
│                                          │
└──────────────────────────────────────────┘
```

## Design Principles

### 1. Separation of Concerns

**flowctl** orchestrates, **components** process:

```
flowctl Responsibilities:
• Start/stop components
• Monitor health
• Route data
• Handle failures

Component Responsibilities:
• Produce/transform/consume data
• Register with control plane
• Report health status
• Handle business logic
```

### 2. Loose Coupling

Components are **independent processes** connected via gRPC:

- Components don't know about each other
- flowctl manages connections based on configuration
- Easy to replace/upgrade individual components
- Language-agnostic (any language with gRPC support)

### 3. Observable by Design

Built-in observability at every level:

```
Component Level:
• Health checks (/health endpoint)
• Heartbeats (every 10s)
• Structured logging

Control Plane Level:
• Component registry
• Status tracking
• Event metrics
• Stream monitoring
```

### 4. Fail-Fast Philosophy

Components should fail immediately on unrecoverable errors:

```
❌ Bad: Log error and continue
if err != nil {
    log.Printf("error: %v", err)
    // Continue anyway
}

✅ Good: Return error immediately
if err != nil {
    return fmt.Errorf("processing failed: %w", err)
}
```

flowctl will detect failure and can restart components.

### 5. Configuration as Code

Pipeline configuration is:
- **Declarative**: Describe what you want, not how to achieve it
- **Version-controlled**: Store in Git alongside code
- **Validated**: CUE schema ensures correctness
- **Portable**: Same config works across environments

### 6. Cloud-Native Ready

Designed for cloud deployment:
- Containerized components
- Service discovery via control plane
- Health checks for orchestrators (K8s, Nomad)
- Metrics for monitoring (Prometheus)
- Horizontal scalability

## Performance Characteristics

### Latency

```
Component Type       Typical Latency
────────────────────────────────────
Source → Processor   < 1ms (local)
                     < 10ms (network)

Processor Transform  Depends on logic
                     (typically < 1ms)

Sink Write          Depends on backend
                     (DB: 1-10ms)
                     (File: < 1ms)
```

### Throughput

```
Pipeline Type        Typical Throughput
──────────────────────────────────────
Simple Linear        10K-100K events/sec
(Source→Processor→Sink)

Fan-Out (1→N)        Limited by slowest sink
                     Use async sinks for high throughput

Fan-In (N→1)         Sum of all sources
                     Limited by processor capacity

Complex DAG          Depends on topology
                     Profile and optimize bottlenecks
```

### Resource Usage

```
Component         Memory      CPU          Network
─────────────────────────────────────────────────
flowctl CLI       50-100 MB   Low (<5%)    Low
Control Plane     20-50 MB    Low (<5%)    Medium

Source            Varies      Varies       Medium-High
                              (depends on  (producing
                              data source) events)

Processor         Varies      Medium-High  Medium
                              (depends on  (forwarding
                              processing)  events)

Sink              Varies      Medium       Low-Medium
                              (depends on  (receiving
                              writes)      events)
```

## Security Considerations

### Communication Security

- gRPC supports TLS/mTLS
- Control plane can require authentication
- Components validate certificates

### Component Isolation

- Process driver: OS-level process isolation
- Docker driver: Container isolation
- Kubernetes driver: Namespace and network policies

### Configuration Security

- Don't commit secrets to YAML
- Use environment variables for sensitive data
- Consider secrets management (Vault, K8s Secrets)

## Debugging Architecture

### Component-Level Debugging

```
1. Check component logs
   → Look for initialization errors

2. Test component standalone
   → Verify it can start independently

3. Check health endpoint
   → curl http://localhost:8088/health

4. Verify registration
   → Check control plane logs for registration messages
```

### Pipeline-Level Debugging

```
1. Enable debug logging
   → flowctl run --log-level=debug pipeline.yaml

2. Check topology
   → Verify inputs match component IDs

3. Monitor event flow
   → Look for "sending event" / "received event" logs

4. Check backpressure
   → Look for slow components affecting upstream
```

## Additional Resources

- **Configuration Guide**: [configuration.md](configuration.md)
- **Building Components**: [building-components.md](building-components.md)
- **Getting Started**: [../examples/getting-started/README.md](../examples/getting-started/README.md)
- **Real-world Example**: https://github.com/withObsrvr/flowctl-sdk/tree/main/examples/contract-events-pipeline

## Questions?

- **GitHub Issues**: https://github.com/withobsrvr/flowctl/issues
- **Documentation**: https://github.com/withobsrvr/flowctl
