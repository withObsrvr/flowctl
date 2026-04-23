# Docker, Translation, and Local Execution

This document collects the Docker- and translation-related material that used to live in the main README.

## Status

These features still exist in the repository, but they are **not the primary supported runtime path** for flowctl today.

The current recommended production/operator path is:
- `flowctl/v1` pipelines
- `process` runtime
- embedded control plane

Treat Docker/container-oriented workflows as **secondary or experimental** unless you have a specific need for them.

---

## Pipeline Translation

flowctl can translate pipeline configurations to different deployment formats:

```bash
# Translate a pipeline to Docker Compose
./bin/flowctl translate -f examples/docker-pipeline.yaml -o docker-compose

# Save the output to a file
./bin/flowctl translate -f examples/docker-pipeline.yaml -o docker-compose --to-file docker-compose.yml

# Add a resource prefix for naming consistency
./bin/flowctl translate -f examples/docker-pipeline.yaml -o docker-compose --prefix myapp

# Specify a container registry
./bin/flowctl translate -f examples/docker-pipeline.yaml -o docker-compose --registry ghcr.io/myorg

# Generate local execution script/config
./bin/flowctl translate -f examples/local-pipeline.yaml -o local --to-file run_pipeline.sh
```

Supported output formats:
- `docker-compose`: Docker Compose YAML
- `local`: Local execution configuration

---

## Local Execution

When using the `local` output format, flowctl generates a configuration for running your pipeline locally.

### Docker Compose-based Local Execution

The Docker Compose-based local generator creates:
1. A Docker Compose configuration file with profile support
2. An environment file with all required variables
3. Proper dependency ordering between components
4. Health check monitoring
5. Volume management for persistent data and logs

```bash
# Generate Docker Compose configuration for local execution
./bin/flowctl translate -f examples/local-pipeline.yaml -o local --to-file docker-compose.yaml

# Start the pipeline
docker compose --profile local up -d

# View logs
docker compose logs -f

# Stop the pipeline
docker compose down
```

### Legacy Bash Script Generator

For compatibility with older workflows, you can still use the bash script generator:

```bash
export FLOWCTL_LOCAL_GENERATOR_TYPE=bash
./bin/flowctl translate -f examples/local-pipeline.yaml -o local --to-file run_pipeline.sh
chmod +x run_pipeline.sh
./run_pipeline.sh
```

The bash script generator creates:
1. A bash script to start and manage pipeline components
2. An environment file with all required variables
3. Proper dependency ordering between components
4. Health check monitoring
5. Process supervision with automatic restart

---

## Docker and Container Troubleshooting

### Docker Permission Errors

If you encounter permission errors when running sandbox or container-oriented workflows, common fixes include:

**Linux:**
- Run with sudo: `sudo flowctl sandbox start ...`
- Add your user to the docker group: `sudo usermod -aG docker $USER`
- Check if Docker service is running: `sudo systemctl status docker`

**macOS:**
- Ensure Docker Desktop is running
- Check Docker Desktop permissions
- Try restarting Docker Desktop

**NixOS:**
- See [docs/nixos-docker-setup.md](docs/nixos-docker-setup.md)
- Quick fix: run with sudo
- Permanent fix: add user to the docker group in `configuration.nix`

### Container Runtime Not Found

If flowctl cannot find Docker or nerdctl:

1. Install Docker: https://docs.docker.com/get-docker/
2. Or install nerdctl: https://github.com/containerd/nerdctl
3. flowctl uses your system-installed container runtime by default

---

## Recommendation

If you are just getting started with flowctl, return to the main [README](../README.md) and use the process runtime first.
