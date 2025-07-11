# Sandbox Setup and Troubleshooting

## Quick Fix for the Current Error

The error you encountered was due to missing configuration files. Here's how to fix it:

### 1. Create the missing data directories:
```bash
mkdir -p data/postgres data/grafana
```

### 2. The prometheus configuration file has been created at:
```
examples/config/prometheus.yml
```

### 3. Run the sandbox again:
```bash
sudo ./bin/flowctl sandbox start --use-system-runtime --services examples/sandbox.yaml --pipeline examples/sandbox-pipeline.yaml
```

## What Was Fixed

1. **Missing Prometheus Configuration**: The `config/prometheus.yml` was a directory instead of a file. Created a proper Prometheus configuration at `examples/config/prometheus.yml` and updated the sandbox.yaml to reference it correctly.

2. **Missing Data Directories**: Created the required data directories for PostgreSQL and Grafana persistence.

## Sandbox Services

The sandbox includes:
- **Redis** (port 6379) - Event queue
- **ClickHouse** (ports 8123, 9000) - Analytics database  
- **Kafka + Zookeeper** (ports 9092, 2181) - Message streaming
- **PostgreSQL** (port 5432) - Relational database
- **Prometheus** (port 9090) - Metrics collection
- **Grafana** (port 3000) - Metrics visualization (admin/admin)

## Common Issues

### Permission Errors
- Use `sudo` to run flowctl if you get Docker permission errors
- Or add your user to the docker group (see docs/nixos-docker-setup.md for NixOS)

### Port Conflicts  
- Stop any services running on the required ports
- Or modify the ports in sandbox.yaml

### Missing Images
- The first run will download Docker images (may take a few minutes)
- Ensure you have internet connectivity

### Volume Mount Errors
- Ensure the data directories exist: `mkdir -p data/postgres data/grafana`
- Check file paths are correct relative to where you run flowctl

## Stopping the Sandbox

```bash
sudo ./bin/flowctl sandbox stop
```

Or manually clean up:
```bash
sudo docker stop $(sudo docker ps -q --filter "network=sandbox_net")
sudo docker rm $(sudo docker ps -aq --filter "network=sandbox_net")
sudo docker network rm sandbox_net
```