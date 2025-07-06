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

3. **Start the sandbox**:
```bash
flowctl sandbox start --pipeline pipeline.yaml --services sandbox.yaml
```

4. **Monitor status**:
```bash
flowctl sandbox status
```

5. **View logs**:
```bash
flowctl sandbox logs
```

6. **Stop when done**:
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

## Commands

### `flowctl sandbox start`

Starts the sandbox environment with all configured services.

**Options:**
- `--pipeline`: Pipeline YAML file (default: `pipeline.yaml`)
- `--services`: Services configuration file (default: `sandbox.yaml`)
- `--backend`: Container runtime (containerd|docker, default: containerd)
- `--watch`: Enable hot reload on file changes
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

## Bundled Runtime

By default, the sandbox uses a bundled version of nerdctl and containerd stored in `~/.flowctl/runtime/`. This provides:

- Zero-dependency setup
- Consistent runtime across environments
- Rootless operation on supported systems

To use your system's container runtime instead, use `--use-system-runtime`.

## Networking

All sandbox services run on a dedicated Docker network (`sandbox_net` by default). Services can communicate with each other using their service names as hostnames.

For example, if you have a Redis service, your pipeline can connect to `redis:6379`.

## Examples

See the `examples/` directory for:
- `sandbox.yaml` - Complete service configuration example
- `sandbox-pipeline.yaml` - Example pipeline using sandbox services

## Troubleshooting

### "No container runtime found"
- Install Docker or nerdctl on your system, or
- Use the bundled runtime (default behavior)

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