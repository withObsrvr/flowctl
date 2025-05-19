# Testing the Translator System

This document provides instructions on how to test the translator system, which allows converting Obsrvr Flow pipeline configurations to deployment formats like Docker Compose.

## Overview

The translator system allows you to:

1. Parse Flow pipeline YAML configurations
2. Validate the pipeline structure
3. Convert the pipeline to a deployment format (currently Docker Compose)
4. Apply or just generate the translated configuration

## Prerequisites

- Go 1.19 or later
- Make sure you have built the `flowctl` binary

## Testing Instructions

### 1. Building the CLI

If you haven't already built the CLI, run:

```bash
cd /path/to/obsrvr/flowctl
go build -o bin/flowctl main.go
```

### 2. Translating a Pipeline to Docker Compose

To translate a pipeline configuration to Docker Compose:

```bash
./bin/flowctl apply -f examples/docker-test.yaml --target docker-compose --translate-only
```

This will generate a `docker-compose.yml` file in the same directory as the input file. You can specify a different output path using the `-o` flag:

```bash
./bin/flowctl apply -f examples/docker-test.yaml --target docker-compose -o /path/to/output/docker-compose.yml --translate-only
```

### 3. Testing the Generated Docker Compose

Once you have the Docker Compose file, you can test it with Docker Compose:

```bash
cd /directory/containing/docker-compose.yml
docker-compose up
```

This will start the services defined in the Docker Compose file.

### 4. Validation

Ensure that:

1. All services defined in the pipeline are included in the Docker Compose file
2. Service dependencies are correctly set up
3. Environment variables from the pipeline configuration are correctly passed to the services
4. Volumes and ports are correctly configured
5. Networks are correctly set up

## Troubleshooting

If you encounter issues:

1. Run with debug logging to see more details:

```bash
./bin/flowctl apply -f examples/docker-test.yaml --target docker-compose --translate-only --log-level debug
```

2. Check the generated Docker Compose file for syntax errors:

```bash
docker-compose config
```

3. If services fail to start, check the Docker logs:

```bash
docker-compose logs
```

## Next Steps

Once the Docker Compose implementation is working correctly, the next steps will be:

1. Implementing the Kubernetes format generator
2. Adding more configuration options for the generators
3. Enhancing the validation rules for pipeline configurations