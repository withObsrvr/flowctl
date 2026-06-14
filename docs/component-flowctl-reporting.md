# Component flowctl reporting

Data-plane components can report bounded historical work-unit state to the flowctl control plane. This is the contract used by historical ledger workers such as Bronze and Silver loaders.

## Environment contract

```text
ENABLE_FLOWCTL=true
FLOWCTL_ENDPOINT=<host:port>
FLOWCTL_COMPONENT_ID=<stable component id>
FLOWCTL_RUN_ID=<pipeline run id>
FLOWCTL_ATTEMPT=<attempt number>
FLOWCTL_HEARTBEAT_INTERVAL_MS=10000
```

`flowctl run` injects `FLOWCTL_ENDPOINT`, `FLOWCTL_COMPONENT_ID`, `FLOWCTL_RUN_ID`, `FLOWCTL_ATTEMPT`, and heartbeat interval values for process-managed components.

Historical range workers should also accept:

```text
START_LEDGER=<inclusive start>
END_LEDGER=<inclusive end>
CHUNK_START=<inclusive chunk start>
CHUNK_END=<inclusive chunk end>
```

## Go helper

Use `github.com/withobsrvr/flowctl/pkg/component` from a component binary:

```go
cfg := component.ConfigFromEnv()
reporter, err := component.NewReporter(ctx, cfg)
if err != nil {
    return err
}
defer reporter.Close()

if err := reporter.Register(ctx, flowctlpb.ServiceType_SERVICE_TYPE_SOURCE, map[string]string{
    "pipeline": "obsrvr-mainnet-bronze-repair",
    "network":  "pubnet",
}); err != nil {
    return err
}

go reporter.StartHeartbeatLoop(ctx, func() map[string]float64 {
    return map[string]float64{"ledgers_processed": float64(processed)}
}, nil)

_ = reporter.ReportChunkProgress(ctx, chunkStart, chunkEnd, "ducklake_push", nil, nil)

_ = reporter.ReportChunkCompleted(ctx, chunkStart, chunkEnd, true, map[string]int64{
    "ledgers_row_v2":      250000,
    "transactions_row_v2": 22685377,
}, map[string]string{
    "gate":   "bronze-silver-readiness-direct",
    "passed": "true",
})
```

On failure:

```go
_ = reporter.ReportChunkFailed(
    ctx,
    chunkStart,
    chunkEnd,
    "verification",
    flowctlpb.FailureClass_FAILURE_CLASS_RETRYABLE_INFRASTRUCTURE,
    "catalog postgres connection timed out",
    "retry_verification",
)
```

## Operator inspection

```bash
flowctl chunks list --run <run-id>
flowctl chunks list --run <run-id> --status failed
flowctl chunks show <chunk-id>
```
