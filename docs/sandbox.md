# Flowctl Sandbox

The Flowctl Sandbox provides a local-first developer experience for running Flowctl pipelines and their supporting infrastructure in an isolated containerized environment.

## Quick Start

1. **Create a sandbox configuration** (`sandbox.yaml`):
```yaml
services:
  redis:
    image: redis:7
    ports:
      - host: 6379
        container: 6379

  clickhouse:
    image: clickhouse/clickhouse-server:latest
    ports:
      - host: 8123
        container: 8123
      - host: 9000
        container: 9000
    env:
      CLICKHOUSE_DB: flow
```

2. **Create a pipeline** (`pipeline.yaml`):
```yaml
apiVersion: flowctl.withobsrvr.com/v1
kind: Pipeline
metadata:
  name: example
spec:
  sources:
    - name: redis-source
      type: redis
      config:
        host: redis:6379
  # ... rest of pipeline
```

3. **Start the sandbox** (services-only mode recommended):
```bash
# Start infrastructure services only (recommended):
flowctl sandbox start --services-only --use-system-runtime --services sandbox.yaml

# If you prefer Docker explicitly:
flowctl sandbox start --services-only --use-system-runtime --backend docker --services sandbox.yaml
```

4. **Run your pipeline** against the sandbox services:
```bash
# Use a host-compatible pipeline configuration
./bin/flowctl run examples/sandbox-host-pipeline.yaml

# Or create your own with localhost endpoints
./bin/flowctl run your-pipeline.yaml
```

5. **Monitor status**:
```bash
flowctl sandbox status
```

6. **View logs**:
```bash
flowctl sandbox logs
```

7. **Stop when done**:
```bash
flowctl sandbox stop
```

## Configuration

### Services Configuration (`sandbox.yaml`)

The services configuration defines the supporting infrastructure for your pipeline:

- **image**: Container image to use
- **ports**: Port mappings between host and container
- **env**: Environment variables
- **volumes**: Volume mounts
- **depends_on**: Service dependencies
- **command**: Custom command to run

### Example with all options:
```yaml
services:
  postgres:
    image: postgres:15
    ports:
      - host: 5432
        container: 5432
    env:
      POSTGRES_DB: flowctl
      POSTGRES_USER: flowctl
    volumes:
      - host: ./data/postgres
        container: /var/lib/postgresql/data
    depends_on:
      - redis
    command: ["postgres", "-c", "log_statement=all"]
```

## Sandbox Modes

The sandbox supports two modes of operation:

### Services-Only Mode (Recommended)

In services-only mode, the sandbox starts infrastructure services and provides connection information for running pipelines on your host machine. This is the recommended approach because:

- **Faster development**: No container overhead for your pipeline logic
- **Better debugging**: Direct access to logs and debugging tools
- **Simpler setup**: No need for flowctl Docker images
- **Platform agnostic**: Works the same across different container runtimes

```bash
flowctl sandbox start --services-only --use-system-runtime --services sandbox.yaml
./bin/flowctl run examples/sandbox-host-pipeline.yaml
```

### Full Mode (Legacy)

Full mode attempts to run your pipeline inside a container alongside the services. This requires a flowctl Docker image and is generally not recommended for development.

```bash
flowctl sandbox start --use-system-runtime --pipeline pipeline.yaml --services sandbox.yaml
```

## Commands

### `flowctl sandbox start`

Starts the sandbox environment with all configured services.

**Options:**
- `--services-only`: Start only infrastructure services, skip pipeline execution (recommended)
- `--pipeline`: Pipeline YAML file (default: `pipeline.yaml`) - not needed in services-only mode
- `--services`: Services configuration file (default: `sandbox.yaml`)
- `--backend`: Container runtime (containerd|docker, default: containerd)
- `--watch`: Enable hot reload on file changes (full mode only)
- `--env-file`: Load environment variables from file
- `--network`: Container network name (default: sandbox_net)
- `--use-system-runtime`: Use system container runtime instead of bundled

### `flowctl sandbox status`

Shows the status of all running sandbox containers, including health, ports, and uptime.

### `flowctl sandbox logs`

Stream logs from sandbox services.

**Options:**
- `[service-name]`: Show logs from specific service (optional)
- `--follow, -f`: Follow log output
- `--tail`: Number of lines to show from end

### `flowctl sandbox stop`

Gracefully stops all sandbox containers and cleans up resources.

**Options:**
- `--timeout`: Graceful shutdown timeout in seconds (default: 30)
- `--force`: Force stop without graceful shutdown

### `flowctl sandbox upgrade`

Upgrades the bundled container runtime (nerdctl + containerd).

**Options:**
- `--version`: Specific version to upgrade to
- `--force`: Force upgrade even if at target version

## Hot Reload

When `--watch` is enabled, the sandbox monitors your pipeline file for changes and automatically restarts the pipeline container when modifications are detected.

This enables rapid development and testing cycles without manual restarts.

## Container Runtime

The sandbox requires a container runtime (Docker or nerdctl) to manage containers.

### Using System Runtime (Recommended)

Currently, you should use your system-installed container runtime with the `--use-system-runtime` flag:

```bash
flowctl sandbox start --use-system-runtime --pipeline pipeline.yaml --services sandbox.yaml
```

The sandbox will automatically detect and use:
- Docker (if installed)
- nerdctl (if installed)

### Bundled Runtime (Coming Soon)

In the future, flowctl will support downloading and managing its own bundled nerdctl/containerd runtime. This feature is currently under development. Until then, please use the `--use-system-runtime` flag.

### NixOS Users

NixOS users must use `--use-system-runtime` due to the unique way NixOS handles binary paths. Install Docker or nerdctl through your Nix configuration:

```nix
# In your configuration.nix or home.nix
environment.systemPackages = with pkgs; [
  docker
  # or
  nerdctl
];
```

## Networking

All sandbox services run on a dedicated Docker network (`sandbox_net` by default). Services can communicate with each other using their service names as hostnames.

For example, if you have a Redis service, your pipeline can connect to `redis:6379`.

## Service Endpoints

When running in services-only mode, the following endpoints are available on your host:

| Service | Endpoint | Notes |
|---------|----------|-------|
| Redis | `localhost:6379` | Key-value store |
| ClickHouse | `localhost:8123` (HTTP)<br>`localhost:9000` (Native) | Analytics database |
| PostgreSQL | `localhost:5432` | Relational database<br>DB: `flowctl`, User: `flowctl` |
| Kafka | `localhost:9092` | Message streaming |
| Zookeeper | `localhost:2181` | Kafka coordination |
| Prometheus | `localhost:9090` | Metrics collection<br>Web UI available |
| Grafana | `localhost:3000` | Visualization<br>Login: admin/admin |

## Pipeline Configuration for Host Mode

When using services-only mode, configure your pipelines to use `localhost` endpoints:

```yaml
sources:
  - name: redis-source
    type: redis
    config:
      host: localhost:6379  # Instead of redis:6379
      
sinks:
  - name: clickhouse-sink
    type: clickhouse
    config:
      host: localhost:9000  # Instead of clickhouse:9000
```

## Examples

See the `examples/` directory for:
- `sandbox.yaml` - Complete service configuration example
- `sandbox-pipeline.yaml` - Example pipeline using container service names
- `sandbox-host-pipeline.yaml` - Example pipeline using localhost endpoints (for services-only mode)
- `sandbox-minimal-host.yaml` - Minimal test pipeline for services-only mode

## Troubleshooting

### "No container runtime found" or "no such file or directory"
- Install Docker or nerdctl on your system
- Always use the `--use-system-runtime` flag (bundled runtime is not yet available)
- For NixOS users: See the NixOS section above for installation instructions

### "Network already exists" 
- Stop the sandbox completely: `flowctl sandbox stop`
- Manually remove the network: `docker network rm sandbox_net`

### "Permission denied"
- Ensure your user can access Docker/containerd
- On Linux, add your user to the `docker` group
- Consider using rootless mode

### File watching not working
- Ensure the pipeline file path is absolute or relative to current directory
- Check file system permissions
- Some filesystems don't support inotify (use polling-based alternatives)